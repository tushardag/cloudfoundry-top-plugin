// Copyright (c) 2017 ECS Team, Inc. - All Rights Reserved
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

package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ecsteam/cloudfoundry-top-plugin/config"
	"github.com/ecsteam/cloudfoundry-top-plugin/toplog"
)

type ResponseError struct {
	Code        int    `json:"code"`
	Description string `json:"description"`
	ErrorCode   string `json:"error_code"`
}

type V2MetadataManager interface {
	MetadataManager
	LoadAllItemsInternal() ([]IMetadata, error)

	CreateResponseObject() IResponse
	CreateResourceObject() IResource
	CreateMetadataEntityObject(guid string) IMetadata
	ProcessResponse(IResponse, []IMetadata) []IMetadata
	ProcessResource(resource IResource) IMetadata
	GetNextUrl(response IResponse) string
	Count(response IResponse) int
	PostProcessLoad([]IMetadata, error)
}

type CommonV2ResponseManager struct {
	*CommonMetadataManager
	mm V2MetadataManager

	autoFullLoadIfNotFound bool
	fullLoadCacheTime      time.Time
	loadInProgress         bool
}

func NewCommonV2ResponseManager(mdGlobalManager MdGlobalManagerInterface,
	dataType DataType,
	url string,
	mm V2MetadataManager,
	autoFullLoadIfNotFound bool) *CommonV2ResponseManager {

	commonV2ResponseMgr := &CommonV2ResponseManager{mm: mm, autoFullLoadIfNotFound: autoFullLoadIfNotFound}
	commonV2ResponseMgr.CommonMetadataManager = NewCommonMetadataManager(mdGlobalManager, dataType, url, mm, DefaultMinimumReloadDuration)
	return commonV2ResponseMgr
}

func (commonV2ResponseMgr *CommonV2ResponseManager) MetadataLoadMethod(guid string) error {
	if guid == ALL {
		return commonV2ResponseMgr.LoadAllItems()
	} else {
		return commonV2ResponseMgr.LoadItem(guid)
	}
}

func (commonV2ResponseMgr *CommonV2ResponseManager) FindItemInternal(guid string, requestLoadIfNotFound bool, createEmptyObjectIfNotFound bool) IMetadata {

	requestLoadIfNotFound = requestLoadIfNotFound && !commonV2ResponseMgr.autoFullLoadIfNotFound
	item, found := commonV2ResponseMgr.CommonMetadataManager.FindItemInternal(guid, requestLoadIfNotFound, createEmptyObjectIfNotFound)

	if commonV2ResponseMgr.autoFullLoadIfNotFound && !found {
		commonV2ResponseMgr.LoadAllItemsAysnc()
	}
	return item
}

func (commonV2ResponseMgr *CommonV2ResponseManager) LoadAllItemsInternal() ([]IMetadata, error) {
	return commonV2ResponseMgr.GetMetadata()
}

func (commonV2ResponseMgr *CommonV2ResponseManager) LoadItemInternal(guid string) (IMetadata, error) {
	url := commonV2ResponseMgr.url + "/" + guid
	now := time.Now()

	outputStr, err := CallAPI(commonV2ResponseMgr.mdGlobalManager.GetCliConnection(), url)
	if err != nil {
		emptyApp := commonV2ResponseMgr.mm.NewItemById(guid)
		return emptyApp, err
	}
	outputBytes := []byte(outputStr)
	resource := commonV2ResponseMgr.mm.CreateResourceObject()
	err = json.Unmarshal(outputBytes, resource)
	if err != nil {
		emptyApp := commonV2ResponseMgr.mm.NewItemById(guid)
		return emptyApp, err
	}

	itemMetadata := commonV2ResponseMgr.mm.ProcessResource(resource)
	itemMetadata.SetCacheTime(&now)
	return itemMetadata, nil
}

func (commonV2ResponseMgr *CommonV2ResponseManager) LoadAllItems() error {
	now := time.Now()
	_, err := commonV2ResponseMgr.LoadAllItemsInternal()
	if err != nil {
		toplog.Warn("*** app metadata error: %v", err.Error())
		return err
	}

	// Loop through existing cache map checking if cacheTime older then "now".
	var deleteMetadataItems []IMetadata
	for _, metadataItem := range commonV2ResponseMgr.MetadataMap {
		cacheTime := metadataItem.GetCacheTime()
		if cacheTime != nil && cacheTime.Before(now) {
			deleteMetadataItems = append(deleteMetadataItems, metadataItem)
		}
	}

	// removing anything that has old
	for _, metadataItem := range deleteMetadataItems {
		id := metadataItem.GetGuid()
		toplog.Info("Delete from cache: %v", id)
		commonV2ResponseMgr.DeleteItem(id)
	}

	return nil
}

func (commonV2ResponseMgr *CommonV2ResponseManager) LoadAllItemsAysnc() {

	commonV2ResponseMgr.MetadataMapMutex.Lock()
	defer commonV2ResponseMgr.MetadataMapMutex.Unlock()

	if commonV2ResponseMgr.loadInProgress {
		toplog.Debug("CommonV2ResponseManager.LoadAllItemsAysnc %v loadInProgress", commonV2ResponseMgr.url)
		return
	}

	commonV2ResponseMgr.loadInProgress = true
	loadAsync := func() {
		toplog.Debug("CommonV2ResponseManager.LoadAllItemsAysnc %v loadAsync thread started", commonV2ResponseMgr.url)
		commonV2ResponseMgr.LoadAllItems()
		toplog.Debug("CommonV2ResponseManager.LoadAllItemsAysnc %v loadAsync thread complete", commonV2ResponseMgr.url)
		commonV2ResponseMgr.loadInProgress = false
	}
	go loadAsync()
}

func (commonMgr *CommonMetadataManager) GetNextUrl(response IResponse) string {
	nextUrl, _ := GetStringValueByFieldName(response, "NextUrl")
	return nextUrl
}

func (commonMgr *CommonMetadataManager) Count(response IResponse) int {
	count, _ := GetIntValueByFieldName(response, "Count")
	return int(count)
}

func (commonMgr *CommonMetadataManager) PostProcessLoad(metadataArray []IMetadata, err error) {
	// do nothing by default
}

func (commonV2ResponseMgr *CommonV2ResponseManager) GetMetadata() ([]IMetadata, error) {
	return commonV2ResponseMgr.GetMetadataFromUrl(commonV2ResponseMgr.GetUrl())
}

func (commonV2ResponseMgr *CommonV2ResponseManager) GetMetadataFromUrl(url string) ([]IMetadata, error) {
	metadataArray := []IMetadata{}
	respError := &ResponseError{}

	// Configure the number of records per API call that we get
	url += "?results-per-page=" + strconv.Itoa(config.ResultsPerPage)

	toplog.Debug("URL: %v", url)
	handleRequest := func(outputBytes []byte) (data interface{}, nextUrl string, err error) {
		resp := commonV2ResponseMgr.mm.CreateResponseObject()
		//toplog.Info("outputBytes: %v", string(outputBytes))
		err = json.Unmarshal(outputBytes, &respError)

		if err != nil {
			toplog.Warn("*** %v unmarshal parsing output: %v", url, string(outputBytes[:]))
			return metadataArray, "", err
		}
		if respError.Code > 0 {
			errMsg := fmt.Sprintf("API response error: %+v", respError)
			toplog.Warn("*** %v %v", url, errMsg)
			return metadataArray, "", errors.New(errMsg)
		}

		err = json.Unmarshal(outputBytes, &resp)
		if err != nil {
			toplog.Warn("*** %v unmarshal parsing output: %v", url, string(outputBytes[:]))
			return metadataArray, "", err
		}
		metadataArray = commonV2ResponseMgr.mm.ProcessResponse(resp, metadataArray)

		// Incrementically add records to our metadata cache as they are retrieved.
		// This helps to get usable data when we have 3000+ items to load
		now := time.Now()
		commonV2ResponseMgr.MetadataMapMutex.Lock()
		for _, metadataItem := range metadataArray {
			metadataItem.SetCacheTime(&now)
			commonV2ResponseMgr.MetadataMap[metadataItem.GetGuid()] = metadataItem
		}
		commonV2ResponseMgr.MetadataMapMutex.Unlock()

		commonV2ResponseMgr.mdGlobalManager.SetStatus(
			fmt.Sprintf("%v metadata loading...  %v of %v\nxxx",
				DataTypeDisplay[commonV2ResponseMgr.dataType],
				len(metadataArray),
				commonV2ResponseMgr.mm.Count(resp)))

		nextUrl = commonV2ResponseMgr.mm.GetNextUrl(resp)
		//nextUrl, _ = GetStringValueByFieldName(resp, "NextUrl")
		return resp, nextUrl, nil
	}

	commonV2ResponseMgr.mdGlobalManager.SetStatus(fmt.Sprintf("%v metadata loading...",
		DataTypeDisplay[commonV2ResponseMgr.dataType]))
	toplog.Info(fmt.Sprintf("%v metadata loading", commonV2ResponseMgr.dataType))

	err := CallPagableAPI(commonV2ResponseMgr.mdGlobalManager.GetCliConnection(), url, handleRequest)

	commonV2ResponseMgr.mm.PostProcessLoad(metadataArray, err)

	commonV2ResponseMgr.mdGlobalManager.SetStatus("")

	return metadataArray, err

}

/*
func (commonV2ResponseMgr *CommonV2ResponseManager) GetMetadataV3FromUrl() ([]IMetadata, error) {

	url := commonV2ResponseMgr.GetUrl()
	metadataArray := []IMetadata{}

	handleRequest := func(outputBytes []byte) (data interface{}, nextUrl string, err error) {
		resp := commonV2ResponseMgr.mm.CreateResponseObject().(IResponseV3)
		err = json.Unmarshal(outputBytes, &resp)
		if err != nil {
			toplog.Warn("*** %v unmarshal parsing output: %v", url, string(outputBytes[:]))
			return metadataArray, "", err
		}
		metadataArray = commonV2ResponseMgr.mm.ProcessResponse(resp, metadataArray)
		nextUrl = resp.GetPagination().Next.Href
		return resp, nextUrl, nil
	}

	err := CallPagableAPI(commonV2ResponseMgr.mdGlobalManager.GetCliConnection(), url, handleRequest)

	return metadataArray, err

}
*/
