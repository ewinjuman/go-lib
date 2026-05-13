package main

import (
	"context"

	"github.com/ewinjuman/go-lib/v2/appContext"
	"github.com/ewinjuman/go-lib/v2/examples/helper"
	"github.com/ewinjuman/go-lib/v2/httpclient"
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
	appCtx := appContext.New(context.Background(), helper.GetLogger())
	response := &ResponseData{}
	//var i int
	err := httpclient.Get("http://localhost:3000/template").WithRequestID("setRequestID").
		WithBasicAuth("ewin", "password").
		WithQueryParam(map[string]string{"msisdn": "08123456", "deviceId": "8jdj8j3mmkldk"}).
		Execute().Consume(response)
	if err != nil {
		appCtx.Log().Error(err.Error())
	}
	appCtx.Log().Info("", logger.Interface("result", response.Total))

}
