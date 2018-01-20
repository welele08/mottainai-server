/*

Copyright (C) 2017-2018  Ettore Di Giacinto <mudler@gentoo.org>
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

package tasksapi

import (
	"strconv"

	"github.com/MottainaiCI/mottainai-server/pkg/context"
	"github.com/MottainaiCI/mottainai-server/pkg/db"
	"github.com/MottainaiCI/mottainai-server/pkg/tasks"

	machinery "github.com/RichardKnop/machinery/v1"
)

// TODO: Add dup.

func APICreate(ctx *context.Context, rabbit *machinery.Server, db *database.Database, opts agenttasks.Task) string {
	docID, err := Create(ctx, rabbit, db, opts)
	if err != nil {
		ctx.NotFound()
		return ""
	}
	return docID
}

func Create(ctx *context.Context, rabbit *machinery.Server, db *database.Database, opts agenttasks.Task) (string, error) {

	docID, err := db.CreateTask(map[string]interface{}{
		"source":    opts.Source,
		"script":    opts.Script,
		"yaml":      opts.Yaml,
		"directory": opts.Directory,
		"task":      opts.TaskName,
		"status":    "waiting",
		"output":    "",
		"namespace": opts.Namespace,
		"commit":    opts.Commit,
		"result":    "none",
		"image":     opts.Image,
	})

	if err != nil {
		return "", err
	}
	SendTask(db, rabbit, docID)

	//ctx.Redirect("/tasks")
	//ctx.Redirect("/tasks/display/" + strconv.Itoa(docID))
	//ShowAll(ctx, db)
	return strconv.Itoa(docID), nil
}