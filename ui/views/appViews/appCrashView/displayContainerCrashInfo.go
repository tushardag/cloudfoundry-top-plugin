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

package appCrashView

import (
	"fmt"

	"github.com/ecsteam/cloudfoundry-top-plugin/metadata/crashData"
)

type DisplayContainerCrashInfo struct {
	*crashData.ContainerCrashInfo

	CrashTimeFormatted string
	key                string
}

func NewDisplayContainerCrashInfo(containerCrashInfo *crashData.ContainerCrashInfo) *DisplayContainerCrashInfo {
	crashInfo := &DisplayContainerCrashInfo{}
	crashInfo.ContainerCrashInfo = containerCrashInfo
	return crashInfo
}

func (cs *DisplayContainerCrashInfo) Id() string {
	if cs.key == "" {
		cs.key = fmt.Sprintf("%v-%v", cs.CrashTime.Format("01022006150405"), cs.ContainerIndex)
	}
	return cs.key
}
