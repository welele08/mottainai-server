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

package namespacesapi

import (
	"github.com/MottainaiCI/mottainai-server/pkg/context"
	setting "github.com/MottainaiCI/mottainai-server/pkg/settings"
	"github.com/go-macaron/binding"

	macaron "gopkg.in/macaron.v1"
)

func Setup(m *macaron.Macaron) {
	m.Invoke(func(config *setting.Config) {
		//bind := binding.Bind
		reqSignIn := context.Toggle(&context.ToggleOptions{
			SignInRequired: true,
			Config:         config,
			BaseURL:        config.GetWeb().AppSubURL})
		//reqAdmin := context.Toggle(&context.ToggleOptions{AdminRequired: true})

		m.Group(config.GetWeb().GroupAppPath(), func() {
			m.Get("/api/namespace/list", reqSignIn, NamespaceList)
			m.Get("/api/namespace/:name/list", reqSignIn, NamespaceListArtefacts)
			m.Get("/api/namespace/:name/create", reqSignIn, NamespaceCreate)
			m.Get("/api/namespace/:name/delete", reqSignIn, NamespaceDelete)
			m.Get("/api/namespace/:name/tag/:taskid", reqSignIn, NamespaceTag)
			m.Get("/api/namespace/:name/append/:taskid", reqSignIn, NamespaceAppend)
			m.Post("/api/namespace/remove", reqSignIn, binding.Bind(RemoveForm{}), NamespaceRemovePath)

			m.Get("/api/namespace/:name/clone/:from", reqSignIn, NamespaceClone)
			m.Post("/api/namespace/upload", reqSignIn, binding.MultipartForm(NamespaceForm{}), NamespaceUpload)
		})
	})
}
