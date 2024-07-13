## Otto Share Library
golang base library

#### Feature
1. [Create Error](#error)
2. [Create Logger](#logger)
3. [Create Session](#session)
4. [Http Request Setup](#http-request)
5. [Helper](#helper)

## How to use

```sh
go get github.com/ewinjuman/go-lib
```
- ### Error
    ```
    import "github.com/ewinjuman/go-lib/error"

    err := error.New(400, "Status", "message Error")
    //or
    err = error.NewError(400, "Status", "message is optional")
    ```

- ### Logger
    ```
    import "github.com/ewinjuman/go-lib/logger"

    logOption := logger.Options{
		FileLocation:       "logs/",
		FileName:           "service.log",
		FileTdrLocation:    "logs/service-lite.log",
		FileMaxAge:         30,
		Stdout:             false,
		MaskingLogJsonPath: "pin|token|data.access|data.pin|dll",
	}
    log := logger.New(logOption)
    ```
- ### Session
    ```
    import "github.com/ewinjuman/go-lib/session"

    newSession := session.New(log)
    ```
    you can set other info :
    1. ```newSession.SetInstitutionID("InstitutionI")```
	2. ```newSession.SetAppName("appName")```
	3. ```newSession.SetURL("http://uri.co")```
	4. etc.

    write log
    ```
    newSession.LogRequest("Log Request")
    newSession.LogResponse(response, "Log Response")
    ```
- ### Http Request
    ```
    import REST "github.com/ewinjuman/go-lib/http"

    httpOption := REST.Options{
		Timeout:   20,
		DebugMode: false,
		SkipTLS:   false,
	}
    //new rest http
    newRest:= REST.New(httpOption)

    //execute
    newRest.Execute(newSession, "https://host.com", "path/url", http.MethodPost, nil, request, nil, nil)
    ```
- ### Grpc
    ```
    //on progress
    ```
- ### Helper
    ```
    import ( 
        "github.com/ewinjuman/go-lib/helper"
        "github.com/ewinjuman/go-lib/convert"
        "github.com/ewinjuman/go-lib/helper/codeGenerator"
    )
    ```