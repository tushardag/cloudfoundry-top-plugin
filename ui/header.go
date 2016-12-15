// Copyright (c) 2016 ECS Team, Inc. - All Rights Reserved
// https://github.com/ECSTeam/cloudfoundry-top-plugin
//
// Licensed under the Apache License, Version 2.0 (the "License"); 
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// 
// http://www.apache.org/licenses/LICENSE-2.0
// 
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, 
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ui

import (
	//"fmt"
	"errors"

	"github.com/jroimartin/gocui"
)

type HeaderWidget struct {
	masterUI *MasterUI
	name     string
	height   int
}

func NewHeaderWidget(masterUI *MasterUI, name string, height int) *HeaderWidget {
	return &HeaderWidget{masterUI: masterUI, name: name, height: height}
}

func (w *HeaderWidget) Name() string {
	return w.name
}

func (w *HeaderWidget) Layout(g *gocui.Gui) error {
	maxX, _ := g.Size()
	_, err := g.SetView(w.name, 0, 0, maxX-1, w.height)
	if err != nil {
		if err != gocui.ErrUnknownView {
			return errors.New(w.name + " layout error:" + err.Error())
		}
		//fmt.Fprint(v, w.body)
	}
	return nil
}