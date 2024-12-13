package main

import (
	"github.com/ewinjuman/go-lib/v2/appContext"
	"github.com/ewinjuman/go-lib/v2/examples/helper"
	"github.com/ewinjuman/go-lib/v2/http_v2"
	"github.com/ewinjuman/go-lib/v2/logger"
)

type ResponseData struct {
	TemplatingExample string `json:"Templating example"`
	Users             []struct {
		UserID    string `json:"userId"`
		Firstname string `json:"firstname"`
		Lastname  string `json:"lastname"`
		Friends   []struct {
			ID string `json:"id"`
		} `json:"friends"`
	} `json:"users"`
	Total string `json:"total"`
}

func main() {
	appCtx := appContext.New(helper.GetLogger())
	response := &ResponseData{}
	//var i int
	err := http_v2.Get(appCtx, "http://localhost:3000", "/template").
		WithBasicAuth("ewin", "password").
		WithQueryParam(map[string]string{"msisdn": "08123456", "deviceId": "8jdj8j3mmkldk"}).
		Execute().Consume(&response)
	if err != nil {
		appCtx.Log().Error(appCtx.ToContext(), err.Error())
	}
	appCtx.Log().Info(appCtx.ToContext(), "", logger.Interface("result", response.Total))

}
