/*

Copyright (C) 2018  Ettore Di Giacinto <mudler@gentoo.org>
Credits goes also to Gogs authors, some code portions and re-implemented design
are also coming from the Gogs project, which is using the go-macaron framework
and was really source of ispiration. Kudos to them!

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.

*/

package mottainai

import (
	"fmt"
	"net/http"
	"os"
	"path"

	log "log"

	template "github.com/MottainaiCI/mottainai-server/pkg/template"

	context "github.com/MottainaiCI/mottainai-server/pkg/context"
	database "github.com/MottainaiCI/mottainai-server/pkg/db"
	static "github.com/MottainaiCI/mottainai-server/pkg/static"

	agenttasks "github.com/MottainaiCI/mottainai-server/pkg/tasks"
	machinery "github.com/RichardKnop/machinery/v1"
	config "github.com/RichardKnop/machinery/v1/config"
	"github.com/go-macaron/cache"
	"github.com/go-macaron/csrf"
	"github.com/go-macaron/session"
	cron "github.com/robfig/cron"

	"github.com/go-macaron/captcha"
	rabbithole "github.com/michaelklishin/rabbit-hole"
	macaron "gopkg.in/macaron.v1"

	setting "github.com/MottainaiCI/mottainai-server/pkg/settings"
)

type Mottainai struct {
	*macaron.Macaron
}

func New() *Mottainai {
	return &Mottainai{Macaron: macaron.New()}
}

func Classic(config *setting.Config) *Mottainai {
	cl := macaron.New()
	m := &Mottainai{Macaron: cl}
	m.Map(config)
	cl.Map(config)
	database.NewDatabase("tiedot", config)

	m.Map(database.DBInstance)
	m.Use(macaron.Logger())
	m.Use(macaron.Recovery())

	// TODO: This down deserve config section. Note for _csrf is duplicated in auth
	m.Use(cache.Cacher(cache.Options{ // Name of adapter. Default is "memory".
		Adapter: "memory",
		// Adapter configuration, it's corresponding to adapter.
		AdapterConfig: "",
		// GC interval time in seconds. Default is 60.
		Interval: 60,
		// Configuration section name. Default is "cache".
		Section: "cache",
	}))

	sesopts := session.Options{
		// Name of provider. Default is "memory".
		Provider: "memory",
		// Provider configuration, it's corresponding to provider.
		ProviderConfig: "",
		// Cookie name to save session ID. Default is "MacaronSession".
		CookieName: "MottainaiSession",
		// Cookie path to store. Default is "/".
		CookiePath: "/",
		// GC interval time in seconds. Default is 3600.
		Gclifetime: 3600,
		// Max life time in seconds. Default is whatever GC interval time is.
		Maxlifetime: 3600,
		// Use HTTPS only. Default is false.
		Secure: false,
		// Cookie life time. Default is 0.
		CookieLifeTime: 0,
		// Cookie domain name. Default is empty.
		Domain: "",
		// Session ID length. Default is 16.
		IDLength: 16,
		// Configuration section name. Default is "session".
		Section: "session",
	}

	csrfopts := csrf.Options{ // HTTP header used to set and get token. Default is "X-CSRFToken".
		Header: "X-CSRFToken",
		// Form value used to set and get token. Default is "_csrf".
		Form: "_csrf",
		// Cookie value used to set and get token. Default is "_csrf".
		Cookie: "_csrf",
		// Cookie path. Default is "/".
		CookiePath: "/",
		// Key used for getting the unique ID per user. Default is "uid".
		SessionKey: "uid",
		// If true, send token via header. Default is false.
		SetHeader: false,
		// If true, send token via cookie. Default is false.
		SetCookie: false,
		// Set the Secure flag to true on the cookie. Default is false.
		Secure: false,
		// Disallow Origin appear in request header. Default is false.
		Origin: false,
		// The function called when Validate fails. Default is a simple error print.
		ErrorFunc: func(w http.ResponseWriter) {
			http.Error(w, "Invalid csrf token.", http.StatusBadRequest)
		},
	}

	if config.GetWeb().GetProtocol() == "https" {
		m.Invoke(func(s session.Store) {
			sesopts.Secure = true
			csrfopts.Secure = true
		})
	}

	m.Use(session.Sessioner(sesopts))
	m.Use(csrf.Csrfer())

	// XXX: Workaround
	// Set TMPDIR to /var/tmp by default
	// to prevent large files to be stored in ram instead of disk
	if os.Getenv("TMPDIR") == "" {
		os.Setenv("TMPDIR", "/var/tmp")
	}
	template.Setup(m.Macaron)

	m.Invoke(func(l *log.Logger) {
		l.SetPrefix("[ Mottainai ] ")
	})
	m.Use(captcha.Captchaer(captcha.Options{
		SubURL: config.GetWeb().AppSubURL,
	}))

	m.Use(context.Contexter())
	m.SetStatic()

	if config.GetWeb().EmbedWebHookServer {
		SetupWebHook(m)
	}

	return m
}

func (m *Mottainai) SetStatic() {
	m.Invoke(func(c *setting.Config) {
		m.Use(static.AuthStatic(context.CheckArtefactPermission,
			path.Join(c.GetStorage().ArtefactPath),
			c.GetWeb().AccessControlAllowOrigin, c,
			macaron.StaticOptions{
				Prefix: "artefact",
			},
		))

		m.Use(static.AuthStatic(context.CheckNamespacePermission,
			path.Join(c.GetStorage().NamespacePath),
			c.GetWeb().AccessControlAllowOrigin, c,
			macaron.StaticOptions{
				Prefix: "namespace",
			},
		))
		m.Use(static.AuthStatic(context.CheckStoragePermission,
			path.Join(c.GetStorage().StoragePath),
			c.GetWeb().AccessControlAllowOrigin, c,
			macaron.StaticOptions{
				Prefix: "storage",
			},
		))

		m.Use(static.Static(
			path.Join(c.GetWeb().StaticRootPath, "public"),
			c.GetWeb().AccessControlAllowOrigin, c,
			macaron.StaticOptions{},
		))
	})
}

func (m *Mottainai) listenAddr() string {
	var ans string
	m.Invoke(func(config *setting.Config) {
		ans = fmt.Sprintf("%s:%s", config.GetWeb().HTTPAddr, config.GetWeb().HTTPPort)
	})

	return ans
}
func (m *Mottainai) Url() string {
	return m.url()
}

func (m *Mottainai) url() string {
	var ans string

	m.Invoke(func(config *setting.Config) {
		ans = fmt.Sprintf("%s://%s", config.GetWeb().Protocol,
			m.listenAddr())
	})
	return ans
}

func (m *Mottainai) Start() error {

	m.SetAutoHead(true)

	server := NewServer()

	m.Invoke(func(config *setting.Config) {
		server.Add(config.GetBroker().BrokerDefaultQueue, config)
		if config.GetBroker().Type == "amqp" {
			rmqc, r_error := rabbithole.NewClient(
				config.GetBroker().BrokerURI,
				config.GetBroker().BrokerUser,
				config.GetBroker().BrokerPass)
			if r_error != nil {
				panic(r_error)
			}
			m.Map(rmqc)
		}

		th := agenttasks.DefaultTaskHandler()
		fmt.Println("DB  with " + config.GetDatabase().DBPath)

		c := cron.New()

		m.Map(server)
		m.Map(th)
		m.Map(c)
		m.Map(m)
		m.Map(m.Macaron)
		c.Start()
		m.LoadPlans()
		// For now
		if config.GetWeb().EmbedWebHookServer {
			SetupWebHookAgent(m)
		}

		log.Println("Listen: ", m.url())

		//m.Run()
		var err error
		if len(config.TLSCert) > 0 && len(config.TLSKey) > 0 {
			err = http.ListenAndServeTLS(m.listenAddr(), config.TLSCert, config.TLSKey, m)
		} else {
			err = http.ListenAndServe(m.listenAddr(), m)
		}

		if err != nil {
			log.Fatal(4, "Fail to start server: %v", err)
		}
		c.Stop()
	})

	return nil
}

func (m *Mottainai) WrapF(f http.HandlerFunc) macaron.Handler {
	return func(c *context.Context) {
		f(c.Resp, c.Req.Request)
	}
}

func (m *Mottainai) WrapH(h http.Handler) macaron.Handler {
	return func(c *context.Context) {
		h.ServeHTTP(c.Resp, c.Req.Request)
	}
}
func (m *Mottainai) ProcessPipeline(docID string) (bool, error) {
	result := true
	var rerr error
	m.Invoke(func(d *database.Database, server *MottainaiServer,
		th *agenttasks.TaskHandler, config *setting.Config) {
		pip, err := d.Driver.GetPipeline(docID)
		if err != nil {
			rerr = err
			result = false
			return
		}
		var broker *Broker
		if len(pip.Queue) > 0 {
			broker = server.Get(pip.Queue, config)
			log.Println("Sending pipeline to queue ", pip.Queue)

		} else {
			broker = server.Get(config.GetBroker().BrokerDefaultQueue, config)
			log.Println("Sending pipeline to queue ", config.GetBroker().BrokerDefaultQueue)
		}

		if len(pip.Chord) > 0 {
			tt := make(map[string]string)
			for _, m := range pip.Group {
				tt[pip.Tasks[m].ID] = pip.Tasks[m].TaskName
			}
			cc := make(map[string]string)
			for _, m := range pip.Chord {
				cc[pip.Tasks[m].ID] = pip.Tasks[m].TaskName
			}
			log.Println("Sending chord ")

			_, err := broker.SendChord(&BrokerSendOptions{Retry: pip.Trials(), ChordGroup: cc, Group: tt, Concurrency: pip.Concurrency})
			if err != nil {
				rerr = err
				fmt.Printf("Could not send task: %s", err.Error())
				for _, t := range pip.Tasks {
					d.Driver.UpdateTask(t.ID, map[string]interface{}{
						"result": "error",
						"status": "done",
						"output": "Backend error, could not send task to broker: " + err.Error(),
					})
				}

				result = false
				return
			}
			return
			return
		}

		if len(pip.Group) > 0 {
			tt := make(map[string]string)
			for _, m := range pip.Group {
				tt[pip.Tasks[m].ID] = pip.Tasks[m].TaskName
			}
			log.Println("Sending group ")

			_, err := broker.SendGroup(&BrokerSendOptions{Retry: pip.Trials(), Group: tt, Concurrency: pip.Concurrency})
			if err != nil {
				rerr = err
				fmt.Printf("Could not send group: %s", err.Error())
				for _, t := range pip.Tasks {
					d.Driver.UpdateTask(t.ID, map[string]interface{}{
						"result": "error",
						"status": "done",
						"output": "Backend error, could not send task to broker: " + err.Error(),
					})
				}

				result = false
				return
			}
			return
		}

		if len(pip.Chain) > 0 {
			tt := make(map[string]string)
			for _, m := range pip.Chain {
				tt[pip.Tasks[m].ID] = pip.Tasks[m].TaskName
			}
			log.Println("Sending chain ")

			_, err := broker.SendChain(&BrokerSendOptions{Retry: pip.Trials(), Group: tt, Concurrency: pip.Concurrency})
			if err != nil {
				rerr = err
				log.Println("Could not send task: ", err.Error())

				for _, t := range pip.Tasks {
					d.Driver.UpdateTask(t.ID, map[string]interface{}{
						"result": "error",
						"status": "done",
						"output": "Backend error, could not send task to broker: " + err.Error(),
					})
				}

				result = false
				return
			}
			return
		}

	})

	return result, rerr
}

func (m *Mottainai) SendTask(docID string) (bool, error) {
	result := true
	var err error
	m.Invoke(func(d *database.Database, server *MottainaiServer, th *agenttasks.TaskHandler, config *setting.Config) {

		task, err := d.Driver.GetTask(docID)
		if err != nil {
			result = false
			return
		}
		task.ClearBuildLog(config.GetStorage().ArtefactPath)
		var broker *Broker
		if len(task.Queue) > 0 {
			broker = server.Get(task.Queue, config)
			log.Println("Sending task to queue ", task.Queue)

		} else {
			broker = server.Get(config.GetBroker().BrokerDefaultQueue, config)
			log.Println("Sending task to queue ", config.GetBroker().BrokerDefaultQueue)

		}

		d.Driver.UpdateTask(docID, map[string]interface{}{"status": "waiting", "result": "none"})

		fmt.Printf("Task Source: %v, Script: %v, Directory: %v, TaskName: %v", task.Source, task.Script, task.Directory, task.TaskName)

		if !th.Exists(task.TaskName) {
			fmt.Printf("Could not send task: Invalid task name")
			result = false
			return
		}

		_, err = broker.SendTask(&BrokerSendOptions{Retry: task.Trials(), Delayed: task.Delayed, TaskName: task.TaskName, TaskID: docID})
		if err != nil {
			fmt.Printf("Could not send task: %s", err.Error())
			d.Driver.UpdateTask(docID, map[string]interface{}{
				"result": "error",
				"status": "done",
				"output": "Backend error, could not send task to broker: " + err.Error(),
			})

			result = false
			return
		}
	})
	return result, err
}

func (m *Mottainai) LoadPlans() {
	m.Invoke(func(c *cron.Cron, d *database.Database) {

		for _, plan := range d.Driver.AllPlans() {
			fmt.Println("Loading plan: ", plan.Task, plan)
			id := plan.ID
			c.AddFunc(plan.Planned, func() {
				plan, _ := d.Driver.GetPlan(id)
				plan.Task.Reset()
				docID, _ := d.Driver.CreateTask(plan.Task.ToMap())
				m.SendTask(docID)
			})
		}

	})
}

func (m *Mottainai) ReloadCron() {

	m.Invoke(func(c *cron.Cron, d *database.Database) {
		c.Stop()
		c = cron.New()
		m.Map(c)
		m.LoadPlans()
		c.Start()
	})

}

func NewMachineryServer(queue string, settings *setting.Config) (*machinery.Server, error) {

	var amqpConfig *config.AMQPConfig
	if settings.GetBroker().Type == "amqp" {
		amqpConfig = &config.AMQPConfig{
			Exchange:     settings.GetBroker().BrokerExchange,
			ExchangeType: settings.GetBroker().BrokerExchangeType,
			BindingKey:   queue + "_key",
			//BindingKey:   settings.BrokerBindingKey,
		}

	}
	var cnf = &config.Config{
		Broker:          settings.GetBroker().Broker,
		DefaultQueue:    queue,
		ResultBackend:   settings.GetBroker().BrokerResultBackend,
		ResultsExpireIn: settings.GetBroker().ResultsExpireIn,
		AMQP:            amqpConfig,
	}

	server, err := machinery.NewServer(cnf)
	return server, err
}
