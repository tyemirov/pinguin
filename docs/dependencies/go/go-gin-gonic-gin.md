# github.com/gin-gonic/gin (v1.11.0)

## doc.md

# Gin Quick Start

## Contents

- [Build Tags](#build-tags)
  - [Build with json replacement](#build-with-json-replacement)
  - [Build without `MsgPack` rendering feature](#build-without-msgpack-rendering-feature)
- [API Examples](#api-examples)
  - [Using GET, POST, PUT, PATCH, DELETE and OPTIONS](#using-get-post-put-patch-delete-and-options)
  - [Parameters in path](#parameters-in-path)
  - [Querystring parameters](#querystring-parameters)
  - [Multipart/Urlencoded Form](#multiparturlencoded-form)
  - [Another example: query + post form](#another-example-query--post-form)
  - [Map as querystring or postform parameters](#map-as-querystring-or-postform-parameters)
  - [Upload files](#upload-files)
    - [Single file](#single-file)
    - [Multiple files](#multiple-files)
  - [Grouping routes](#grouping-routes)
  - [Blank Gin without middleware by default](#blank-gin-without-middleware-by-default)
  - [Using middleware](#using-middleware)
  - [Custom Recovery behavior](#custom-recovery-behavior)
  - [How to write log file](#how-to-write-log-file)
  - [Custom Log Format](#custom-log-format)
  - [Controlling Log output coloring](#controlling-log-output-coloring)
  - [Model binding and validation](#model-binding-and-validation)
  - [Custom Validators](#custom-validators)
  - [Only Bind Query String](#only-bind-query-string)
  - [Bind Query String or Post Data](#bind-query-string-or-post-data)
  - [Bind default value if none provided](#bind-default-value-if-none-provided)
  - [Collection format for arrays](#collection-format-for-arrays)
  - [Bind Uri](#bind-uri)
  - [Bind custom unmarshaler](#bind-custom-unmarshaler)
  - [Bind Header](#bind-header)
  - [Bind HTML checkboxes](#bind-html-checkboxes)
  - [Multipart/Urlencoded binding](#multiparturlencoded-binding)
  - [XML, JSON, YAML, TOML and ProtoBuf rendering](#xml-json-yaml-toml-and-protobuf-rendering)
    - [SecureJSON](#securejson)
    - [JSONP](#jsonp)
    - [AsciiJSON](#asciijson)
    - [PureJSON](#purejson)
  - [Serving static files](#serving-static-files)
  - [Serving data from file](#serving-data-from-file)
  - [Serving data from reader](#serving-data-from-reader)
  - [HTML rendering](#html-rendering)
    - [Custom Template renderer](#custom-template-renderer)
    - [Custom Delimiters](#custom-delimiters)
    - [Custom Template Funcs](#custom-template-funcs)
  - [Multitemplate](#multitemplate)
  - [Redirects](#redirects)
  - [Custom Middleware](#custom-middleware)
  - [Using BasicAuth() middleware](#using-basicauth-middleware)
  - [Goroutines inside a middleware](#goroutines-inside-a-middleware)
  - [Custom HTTP configuration](#custom-http-configuration)
  - [Support Let's Encrypt](#support-lets-encrypt)
  - [Run multiple service using Gin](#run-multiple-service-using-gin)
  - [Graceful shutdown or restart](#graceful-shutdown-or-restart)
    - [Third-party packages](#third-party-packages)
    - [Manually](#manually)
  - [Build a single binary with templates](#build-a-single-binary-with-templates)
  - [Bind form-data request with custom struct](#bind-form-data-request-with-custom-struct)
  - [Try to bind body into different structs](#try-to-bind-body-into-different-structs)
  - [Bind form-data request with custom struct and custom tag](#bind-form-data-request-with-custom-struct-and-custom-tag)
  - [http2 server push](#http2-server-push)
  - [Define format for the log of routes](#define-format-for-the-log-of-routes)
  - [Set and get a cookie](#set-and-get-a-cookie)
  - [Custom json codec at runtime](#custom-json-codec-at-runtime)
- [Don't trust all proxies](#dont-trust-all-proxies)
- [Testing](#testing)

## Build tags

### Build with json replacement

Gin uses `encoding/json` as the default JSON package but you can change it by building from other tags.

[jsoniter](https://github.com/json-iterator/go)

```sh
go build -tags=jsoniter .
```

[go-json](https://github.com/goccy/go-json)

```sh
go build -tags=go_json .
```

[sonic](https://github.com/bytedance/sonic)

```sh
$ go build -tags=sonic .
```

### Build without `MsgPack` rendering feature

Gin enables `MsgPack` rendering feature by default. But you can disable this feature by specifying `nomsgpack` build tag.

```sh
go build -tags=nomsgpack .
```

This is useful to reduce the binary size of executable files. See the [detail information](https://github.com/gin-gonic/gin/pull/1852).

## API Examples

You can find a number of ready-to-run examples at [Gin examples repository](https://github.com/gin-gonic/examples).

### Using GET, POST, PUT, PATCH, DELETE and OPTIONS

```go
func main() {
  // Creates a gin router with default middleware:
  // logger and recovery (crash-free) middleware
  router := gin.Default()

  router.GET("/someGet", getting)
  router.POST("/somePost", posting)
  router.PUT("/somePut", putting)
  router.DELETE("/someDelete", deleting)
  router.PATCH("/somePatch", patching)
  router.HEAD("/someHead", head)
  router.OPTIONS("/someOptions", options)

  // By default, it serves on :8080 unless a
  // PORT environment variable was defined.
  router.Run()
  // router.Run(":3000") for a hard coded port
}
```

### Parameters in path

```go
func main() {
  router := gin.Default()

  // This handler will match /user/john but will not match /user/ or /user
  router.GET("/user/:name", func(c *gin.Context) {
    name := c.Param("name")
    c.String(http.StatusOK, "Hello %s", name)
  })

  // However, this one will match /user/john/ and also /user/john/send
  // If no other routers match /user/john, it will redirect to /user/john/
  router.GET("/user/:name/*action", func(c *gin.Context) {
    name := c.Param("name")
    action := c.Param("action")
    message := name + " is " + action
    c.String(http.StatusOK, message)
  })

  // For each matched request Context will hold the route definition
  router.POST("/user/:name/*action", func(c *gin.Context) {
    b := c.FullPath() == "/user/:name/*action" // true
    c.String(http.StatusOK, "%t", b)
  })

  // This handler will add a new router for /user/groups.
  // Exact routes are resolved before param routes, regardless of the order they were defined.
  // Routes starting with /user/groups are never interpreted as /user/:name/... routes
  router.GET("/user/groups", func(c *gin.Context) {
    c.String(http.StatusOK, "The available groups are [...]")
  })

  router.Run(":8080")
}
```

### Querystring parameters

```go
func main() {
  router := gin.Default()

  // Query string parameters are parsed using the existing underlying request object.
  // The request responds to a URL matching: /welcome?firstname=Jane&lastname=Doe
  router.GET("/welcome", func(c *gin.Context) {
    firstname := c.DefaultQuery("firstname", "Guest")
    lastname := c.Query("lastname") // shortcut for c.Request.URL.Query().Get("lastname")

    c.String(http.StatusOK, "Hello %s %s", firstname, lastname)
  })
  router.Run(":8080")
}
```

### Multipart/Urlencoded Form

```go
func main() {
  router := gin.Default()

  router.POST("/form_post", func(c *gin.Context) {
    message := c.PostForm("message")
    nick := c.DefaultPostForm("nick", "anonymous")

    c.JSON(http.StatusOK, gin.H{
      "status":  "posted",
      "message": message,
      "nick":    nick,
    })
  })
  router.Run(":8080")
}
```

### Another example: query + post form

```sh
POST /post?id=1234&page=1 HTTP/1.1
Content-Type: application/x-www-form-urlencoded

name=manu&message=this_is_great
```

```go
func main() {
  router := gin.Default()

  router.POST("/post", func(c *gin.Context) {

    id := c.Query("id")
    page := c.DefaultQuery("page", "0")
    name := c.PostForm("name")
    message := c.PostForm("message")

    fmt.Printf("id: %s; page: %s; name: %s; message: %s", id, page, name, message)
  })
  router.Run(":8080")
}
```

```sh
id: 1234; page: 1; name: manu; message: this_is_great
```

### Map as querystring or postform parameters

```sh
POST /post?ids[a]=1234&ids[b]=hello HTTP/1.1
Content-Type: application/x-www-form-urlencoded

names[first]=thinkerou&names[second]=tianou
```

```go
func main() {
  router := gin.Default()

  router.POST("/post", func(c *gin.Context) {

    ids := c.QueryMap("ids")
    names := c.PostFormMap("names")

    fmt.Printf("ids: %v; names: %v", ids, names)
  })
  router.Run(":8080")
}
```

```sh
ids: map[b:hello a:1234]; names: map[second:tianou first:thinkerou]
```

### Upload files

#### Single file

References issue [#774](https://github.com/gin-gonic/gin/issues/774) and detail [example code](https://github.com/gin-gonic/examples/tree/master/upload-file/single).

`file.Filename` **SHOULD NOT** be trusted. See [`Content-Disposition` on MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition#Directives) and [#1693](https://github.com/gin-gonic/gin/issues/1693)

> The filename is always optional and must not be used blindly by the application: path information should be stripped, and conversion to the server file system rules should be done.

```go
func main() {
  router := gin.Default()
  // Set a lower memory limit for multipart forms (default is 32 MiB)
  router.MaxMultipartMemory = 8 << 20  // 8 MiB
  router.POST("/upload", func(c *gin.Context) {
    // Single file
    file, _ := c.FormFile("file")
    log.Println(file.Filename)

    // Upload the file to specific dst.
    c.SaveUploadedFile(file, dst)

    c.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", file.Filename))
  })
  router.Run(":8080")
}
```

How to `curl`:

```bash
curl -X POST http://localhost:8080/upload \
  -F "file=@/Users/appleboy/test.zip" \
  -H "Content-Type: multipart/form-data"
```

#### Multiple files

See the detailed [example code](https://github.com/gin-gonic/examples/tree/master/upload-file/multiple).

```go
func main() {
  router := gin.Default()
  // Set a lower memory limit for multipart forms (default is 32 MiB)
  router.MaxMultipartMemory = 8 << 20  // 8 MiB
  router.POST("/upload", func(c *gin.Context) {
    // Multipart form
    form, _ := c.MultipartForm()
    files := form.File["upload[]"]

    for _, file := range files {
      log.Println(file.Filename)

      // Upload the file to specific dst.
      c.SaveUploadedFile(file, dst)
    }
    c.String(http.StatusOK, fmt.Sprintf("%d files uploaded!", len(files)))
  })
  router.Run(":8080")
}
```

How to `curl`:

```bash
curl -X POST http://localhost:8080/upload \
  -F "upload[]=@/Users/appleboy/test1.zip" \
  -F "upload[]=@/Users/appleboy/test2.zip" \
  -H "Content-Type: multipart/form-data"
```

### Grouping routes

```go
func main() {
  router := gin.Default()

  // Simple group: v1
  {
    v1 := router.Group("/v1")
    v1.POST("/login", loginEndpoint)
    v1.POST("/submit", submitEndpoint)
    v1.POST("/read", readEndpoint)
  }

  // Simple group: v2
  {
    v2 := router.Group("/v2")
    v2.POST("/login", loginEndpoint)
    v2.POST("/submit", submitEndpoint)
    v2.POST("/read", readEndpoint)
  }

  router.Run(":8080")
}
```

### Blank Gin without middleware by default

Use

```go
r := gin.New()
```

instead of

```go
// Default With the Logger and Recovery middleware already attached
r := gin.Default()
```

### Using middleware

```go
func main() {
  // Creates a router without any middleware by default
  r := gin.New()

  // Global middleware
  // Logger middleware will write the logs to gin.DefaultWriter even if you set with GIN_MODE=release.
  // By default gin.DefaultWriter = os.Stdout
  r.Use(gin.Logger())

  // Recovery middleware recovers from any panics and writes a 500 if there was one.
  r.Use(gin.Recovery())

  // Per route middleware, you can add as many as you desire.
  r.GET("/benchmark", MyBenchLogger(), benchEndpoint)

  // Authorization group
  // authorized := r.Group("/", AuthRequired())
  // exactly the same as:
  authorized := r.Group("/")
  // per group middleware! in this case we use the custom created
  // AuthRequired() middleware just in the "authorized" group.
  authorized.Use(AuthRequired())
  {
    authorized.POST("/login", loginEndpoint)
    authorized.POST("/submit", submitEndpoint)
    authorized.POST("/read", readEndpoint)

    // nested group
    testing := authorized.Group("testing")
    // visit 0.0.0.0:8080/testing/analytics
    testing.GET("/analytics", analyticsEndpoint)
  }

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

### Custom Recovery behavior

```go
func main() {
  // Creates a router without any middleware by default
  r := gin.New()

  // Global middleware
  // Logger middleware will write the logs to gin.DefaultWriter even if you set with GIN_MODE=release.
  // By default gin.DefaultWriter = os.Stdout
  r.Use(gin.Logger())

  // Recovery middleware recovers from any panics and writes a 500 if there was one.
  r.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
    if err, ok := recovered.(string); ok {
      c.String(http.StatusInternalServerError, fmt.Sprintf("error: %s", err))
    }
    c.AbortWithStatus(http.StatusInternalServerError)
  }))

  r.GET("/panic", func(c *gin.Context) {
    // panic with a string -- the custom middleware could save this to a database or report it to the user
    panic("foo")
  })

  r.GET("/", func(c *gin.Context) {
    c.String(http.StatusOK, "ohai")
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

### How to write log file

```go
func main() {
  // Disable Console Color, you don't need console color when writing the logs to file.
  gin.DisableConsoleColor()

  // Logging to a file.
  f, _ := os.Create("gin.log")
  gin.DefaultWriter = io.MultiWriter(f)

  // Use the following code if you need to write the logs to file and console at the same time.
  // gin.DefaultWriter = io.MultiWriter(f, os.Stdout)

  router := gin.Default()
  router.GET("/ping", func(c *gin.Context) {
      c.String(http.StatusOK, "pong")
  })

   router.Run(":8080")
}
```

### Custom Log Format

```go
func main() {
  router := gin.New()

  // LoggerWithFormatter middleware will write the logs to gin.DefaultWriter
  // By default gin.DefaultWriter = os.Stdout
  router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {

    // your custom format
    return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
        param.ClientIP,
        param.TimeStamp.Format(time.RFC1123),
        param.Method,
        param.Path,
        param.Request.Proto,
        param.StatusCode,
        param.Latency,
        param.Request.UserAgent(),
        param.ErrorMessage,
    )
  }))
  router.Use(gin.Recovery())

  router.GET("/ping", func(c *gin.Context) {
    c.String(http.StatusOK, "pong")
  })

  router.Run(":8080")
}
```

Sample Output

```sh
::1 - [Fri, 07 Dec 2018 17:04:38 JST] "GET /ping HTTP/1.1 200 122.767µs "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/71.0.3578.80 Safari/537.36" "
```

### Skip logging

```go
func main() {
  router := gin.New()

  // skip logging for desired paths by setting SkipPaths in LoggerConfig
  loggerConfig := gin.LoggerConfig{SkipPaths: []string{"/metrics"}}

  // skip logging based on your logic by setting Skip func in LoggerConfig
  loggerConfig.Skip = func(c *gin.Context) bool {
      // as an example skip non server side errors
      return c.Writer.Status() < http.StatusInternalServerError
  }

  router.Use(gin.LoggerWithConfig(loggerConfig))
  router.Use(gin.Recovery())

  // skipped
  router.GET("/metrics", func(c *gin.Context) {
      c.Status(http.StatusNotImplemented)
  })

  // skipped
  router.GET("/ping", func(c *gin.Context) {
      c.String(http.StatusOK, "pong")
  })

  // not skipped
  router.GET("/data", func(c *gin.Context) {
    c.Status(http.StatusNotImplemented)
  })

  router.Run(":8080")
}

```

### Controlling Log output coloring

By default, logs output on console should be colorized depending on the detected TTY.

Never colorize logs:

```go
func main() {
  // Disable log's color
  gin.DisableConsoleColor()

  // Creates a gin router with default middleware:
  // logger and recovery (crash-free) middleware
  router := gin.Default()

  router.GET("/ping", func(c *gin.Context) {
      c.String(http.StatusOK, "pong")
  })

  router.Run(":8080")
}
```

Always colorize logs:

```go
func main() {
  // Force log's color
  gin.ForceConsoleColor()

  // Creates a gin router with default middleware:
  // logger and recovery (crash-free) middleware
  router := gin.Default()

  router.GET("/ping", func(c *gin.Context) {
      c.String(http.StatusOK, "pong")
  })

  router.Run(":8080")
}
```

### Model binding and validation

To bind a request body into a type, use model binding. We currently support binding of JSON, XML, YAML, TOML and standard form values (foo=bar&boo=baz).

Gin uses [**go-playground/validator/v10**](https://github.com/go-playground/validator) for validation. Check the full docs on tags usage [here](https://pkg.go.dev/github.com/go-playground/validator#hdr-Baked_In_Validators_and_Tags).

Note that you need to set the corresponding binding tag on all fields you want to bind. For example, when binding from JSON, set `json:"fieldname"`.

Also, Gin provides two sets of methods for binding:

- **Type** - Must bind
  - **Methods** - `Bind`, `BindJSON`, `BindXML`, `BindQuery`, `BindYAML`, `BindHeader`, `BindTOML`
  - **Behavior** - These methods use `MustBindWith` under the hood. If there is a binding error, the request is aborted with `c.AbortWithError(400, err).SetType(ErrorTypeBind)`. This sets the response status code to 400 and the `Content-Type` header is set to `text/plain; charset=utf-8`. Note that if you try to set the response code after this, it will result in a warning `[GIN-debug] [WARNING] Headers were already written. Wanted to override status code 400 with 422`. If you wish to have greater control over the behavior, consider using the `ShouldBind` equivalent method.
- **Type** - Should bind
  - **Methods** - `ShouldBind`, `ShouldBindJSON`, `ShouldBindXML`, `ShouldBindQuery`, `ShouldBindYAML`, `ShouldBindHeader`, `ShouldBindTOML`,
  - **Behavior** - These methods use `ShouldBindWith` under the hood. If there is a binding error, the error is returned and it is the developer's responsibility to handle the request and error appropriately.

When using the Bind-method, Gin tries to infer the binder depending on the Content-Type header. If you are sure what you are binding, you can use `MustBindWith` or `ShouldBindWith`.

You can also specify that specific fields are required. If a field is decorated with `binding:"required"` and has an empty value when binding, an error will be returned.

```go
// Binding from JSON
type Login struct {
  User     string `form:"user" json:"user" xml:"user" binding:"required"`
  Password string `form:"password" json:"password" xml:"password" binding:"required"`
}

func main() {
  router := gin.Default()

  // Example for binding JSON ({"user": "manu", "password": "123"})
  router.POST("/loginJSON", func(c *gin.Context) {
    var json Login
    if err := c.ShouldBindJSON(&json); err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      return
    }

    if json.User != "manu" || json.Password != "123" {
      c.JSON(http.StatusUnauthorized, gin.H{"status": "unauthorized"})
      return
    }

    c.JSON(http.StatusOK, gin.H{"status": "you are logged in"})
  })

  // Example for binding XML (
  //  <?xml version="1.0" encoding="UTF-8"?>
  //  <root>
  //    <user>manu</user>
  //    <password>123</password>
  //  </root>)
  router.POST("/loginXML", func(c *gin.Context) {
    var xml Login
    if err := c.ShouldBindXML(&xml); err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      return
    }

    if xml.User != "manu" || xml.Password != "123" {
      c.JSON(http.StatusUnauthorized, gin.H{"status": "unauthorized"})
      return
    }

    c.JSON(http.StatusOK, gin.H{"status": "you are logged in"})
  })

  // Example for binding a HTML form (user=manu&password=123)
  router.POST("/loginForm", func(c *gin.Context) {
    var form Login
    // This will infer what binder to use depending on the content-type header.
    if err := c.ShouldBind(&form); err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      return
    }

    if form.User != "manu" || form.Password != "123" {
      c.JSON(http.StatusUnauthorized, gin.H{"status": "unauthorized"})
      return
    }

    c.JSON(http.StatusOK, gin.H{"status": "you are logged in"})
  })

  // Listen and serve on 0.0.0.0:8080
  router.Run(":8080")
}
```

Sample request

```sh
$ curl -v -X POST \
  http://localhost:8080/loginJSON \
  -H 'content-type: application/json' \
  -d '{ "user": "manu" }'
> POST /loginJSON HTTP/1.1
> Host: localhost:8080
> User-Agent: curl/7.51.0
> Accept: */*
> content-type: application/json
> Content-Length: 18
>
* upload completely sent off: 18 out of 18 bytes
< HTTP/1.1 400 Bad Request
< Content-Type: application/json; charset=utf-8
< Date: Fri, 04 Aug 2017 03:51:31 GMT
< Content-Length: 100
<
{"error":"Key: 'Login.Password' Error:Field validation for 'Password' failed on the 'required' tag"}
```

Skip-validation: Running the example above using the `curl` command returns an error. This is because the example uses `binding:"required"` for `Password`. If instead, you use `binding:"-"` for `Password`, then it will not return an error when you run the example again.

### Custom Validators

It is also possible to register custom validators. See the [example code](https://github.com/gin-gonic/examples/tree/master/custom-validation/server.go).

```go
package main

import (
  "net/http"
  "time"

  "github.com/gin-gonic/gin"
  "github.com/gin-gonic/gin/binding"
  "github.com/go-playground/validator/v10"
)

// Booking contains binded and validated data.
type Booking struct {
  CheckIn  time.Time `form:"check_in" binding:"required,bookabledate" time_format:"2006-01-02"`
  CheckOut time.Time `form:"check_out" binding:"required,gtfield=CheckIn" time_format:"2006-01-02"`
}

var bookableDate validator.Func = func(fl validator.FieldLevel) bool {
  date, ok := fl.Field().Interface().(time.Time)
  if ok {
    today := time.Now()
    if today.After(date) {
      return false
    }
  }
  return true
}

func main() {
  route := gin.Default()

  if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
    v.RegisterValidation("bookabledate", bookableDate)
  }

  route.GET("/bookable", getBookable)
  route.Run(":8085")
}

func getBookable(c *gin.Context) {
  var b Booking
  if err := c.ShouldBindWith(&b, binding.Query); err == nil {
    c.JSON(http.StatusOK, gin.H{"message": "Booking dates are valid!"})
  } else {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
  }
}
```

```console
$ curl "localhost:8085/bookable?check_in=2030-04-16&check_out=2030-04-17"
{"message":"Booking dates are valid!"}

$ curl "localhost:8085/bookable?check_in=2030-03-10&check_out=2030-03-09"
{"error":"Key: 'Booking.CheckOut' Error:Field validation for 'CheckOut' failed on the 'gtfield' tag"}

$ curl "localhost:8085/bookable?check_in=2000-03-09&check_out=2000-03-10"
{"error":"Key: 'Booking.CheckIn' Error:Field validation for 'CheckIn' failed on the 'bookabledate' tag"}%
```

[Struct level validations](https://github.com/go-playground/validator/releases/tag/v8.7) can also be registered this way.
See the [struct-lvl-validation example](https://github.com/gin-gonic/examples/tree/master/struct-lvl-validations) to learn more.

### Only Bind Query String

`ShouldBindQuery` function only binds the query params and not the post data. See the [detail information](https://github.com/gin-gonic/gin/issues/742#issuecomment-315953017).

```go
package main

import (
  "log"
  "net/http"

  "github.com/gin-gonic/gin"
)

type Person struct {
  Name    string `form:"name"`
  Address string `form:"address"`
}

func main() {
  route := gin.Default()
  route.Any("/testing", startPage)
  route.Run(":8085")
}

func startPage(c *gin.Context) {
  var person Person
  if c.ShouldBindQuery(&person) == nil {
    log.Println("====== Only Bind By Query String ======")
    log.Println(person.Name)
    log.Println(person.Address)
  }
  c.String(http.StatusOK, "Success")
}

```

### Bind Query String or Post Data

See the [detail information](https://github.com/gin-gonic/gin/issues/742#issuecomment-264681292).

```go
package main

import (
  "log"
  "net/http"
  "time"

  "github.com/gin-gonic/gin"
)

type Person struct {
  Name       string    `form:"name"`
  Address    string    `form:"address"`
  Birthday   time.Time `form:"birthday" time_format:"2006-01-02" time_utc:"1"`
  CreateTime time.Time `form:"createTime" time_format:"unixNano"`
  UnixTime   time.Time `form:"unixTime" time_format:"unix"`
  UnixMilliTime   time.Time `form:"unixMilliTime" time_format:"unixmilli"`
  UnixMicroTime   time.Time `form:"unixMicroTime" time_format:"uNiXmIcRo"` // case does not matter for "unix*" time formats
}

func main() {
  route := gin.Default()
  route.GET("/testing", startPage)
  route.Run(":8085")
}

func startPage(c *gin.Context) {
  var person Person
  // If `GET`, only `Form` binding engine (`query`) used.
  // If `POST`, first checks the `content-type` for `JSON` or `XML`, then uses `Form` (`form-data`).
  // See more at https://github.com/gin-gonic/gin/blob/master/binding/binding.go#L88
  if c.ShouldBind(&person) == nil {
    log.Println(person.Name)
    log.Println(person.Address)
    log.Println(person.Birthday)
    log.Println(person.CreateTime)
    log.Println(person.UnixTime)
    log.Println(person.UnixMilliTime)
    log.Println(person.UnixMicroTime)
  }

  c.String(http.StatusOK, "Success")
}
```

Test it with:

```sh
curl -X GET "localhost:8085/testing?name=appleboy&address=xyz&birthday=1992-03-15&createTime=1562400033000000123&unixTime=1562400033&unixMilliTime=1562400033001&unixMicroTime=1562400033000012"
```


### Bind default value if none provided

If the server should bind a default value to a field when the client does not provide one, specify the default value using the `default` key within the `form` tag:

```go
package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Person struct {
	Name      string    `form:"name,default=William"`
	Age       int       `form:"age,default=10"`
	Friends   []string  `form:"friends,default=Will;Bill"`
	Addresses [2]string `form:"addresses,default=foo bar" collection_format:"ssv"`
	LapTimes  []int     `form:"lap_times,default=1;2;3" collection_format:"csv"`
}

func main() {
	g := gin.Default()
	g.POST("/person", func(c *gin.Context) {
		var req Person
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, err)
			return
		}
		c.JSON(http.StatusOK, req)
	})
	_ = g.Run("localhost:8080")
}
```

```
curl -X POST http://localhost:8080/person
{"Name":"William","Age":10,"Friends":["Will","Bill"],"Colors":["red","blue"],"LapTimes":[1,2,3]}
```

NOTE: For default [collection values](#collection-format-for-arrays), the following rules apply:
- Since commas are used to delimit tag options, they are not supported within a default value and will result in undefined behavior
- For the collection formats "multi" and "csv", a semicolon should be used in place of a comma to delimited default values
- Since semicolons are used to delimit default values for "multi" and "csv", they are not supported within a default value for "multi" and "csv"


#### Collection format for arrays

| Format          | Description                                               | Example                 |
| --------------- | --------------------------------------------------------- | ----------------------- |
| multi (default) | Multiple parameter instances rather than multiple values. | key=foo&key=bar&key=baz |
| csv             | Comma-separated values.                                   | foo,bar,baz             |
| ssv             | Space-separated values.                                   | foo bar baz             |
| tsv             | Tab-separated values.                                     | "foo\tbar\tbaz"         |
| pipes           | Pipe-separated values.                                    | foo\|bar\|baz           |

```go
package main

import (
	"log"
	"time"
	"github.com/gin-gonic/gin"
)

type Person struct {
	Name       string    `form:"name"`
	Addresses  []string  `form:"addresses" collection_format:"csv"`
	Birthday   time.Time `form:"birthday" time_format:"2006-01-02" time_utc:"1"`
	CreateTime time.Time `form:"createTime" time_format:"unixNano"`
	UnixTime   time.Time `form:"unixTime" time_format:"unix"`
}

func main() {
	route := gin.Default()
	route.GET("/testing", startPage)
	route.Run(":8085")
}
func startPage(c *gin.Context) {
	var person Person
	// If `GET`, only `Form` binding engine (`query`) used.
	// If `POST`, first checks the `content-type` for `JSON` or `XML`, then uses `Form` (`form-data`).
	// See more at https://github.com/gin-gonic/gin/blob/master/binding/binding.go#L48
        if c.ShouldBind(&person) == nil {
                log.Println(person.Name)
                log.Println(person.Addresses)
                log.Println(person.Birthday)
                log.Println(person.CreateTime)
                log.Println(person.UnixTime)
        }
	c.String(200, "Success")
}
```

Test it with:
```sh
$ curl -X GET "localhost:8085/testing?name=appleboy&addresses=foo,bar&birthday=1992-03-15&createTime=1562400033000000123&unixTime=1562400033"
```

### Bind Uri

See the [detail information](https://github.com/gin-gonic/gin/issues/846).

```go
package main

import (
  "net/http"

  "github.com/gin-gonic/gin"
)

type Person struct {
  ID string `uri:"id" binding:"required,uuid"`
  Name string `uri:"name" binding:"required"`
}

func main() {
  route := gin.Default()
  route.GET("/:name/:id", func(c *gin.Context) {
    var person Person
    if err := c.ShouldBindUri(&person); err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
      return
    }
    c.JSON(http.StatusOK, gin.H{"name": person.Name, "uuid": person.ID})
  })
  route.Run(":8088")
}
```

Test it with:

```sh
curl -v localhost:8088/thinkerou/987fbc97-4bed-5078-9f07-9141ba07c9f3
curl -v localhost:8088/thinkerou/not-uuid
```

### Bind custom unmarshaler

```go
package main

import (
  "github.com/gin-gonic/gin"
  "strings"
)

type Birthday string

func (b *Birthday) UnmarshalParam(param string) error {
  *b = Birthday(strings.Replace(param, "-", "/", -1))
  return nil
}

func main() {
  route := gin.Default()
  var request struct {
    Birthday Birthday `form:"birthday"`
  }
  route.GET("/test", func(ctx *gin.Context) {
    _ = ctx.BindQuery(&request)
    ctx.JSON(200, request.Birthday)
  })
  route.Run(":8088")
}
```

Test it with:

```sh
curl 'localhost:8088/test?birthday=2000-01-01'
```
Result
```sh
"2000/01/01"
```

### Bind Header

```go
package main

import (
  "fmt"
  "net/http"

  "github.com/gin-gonic/gin"
)

type testHeader struct {
  Rate   int    `header:"Rate"`
  Domain string `header:"Domain"`
}

func main() {
  r := gin.Default()
  r.GET("/", func(c *gin.Context) {
    h := testHeader{}

    if err := c.ShouldBindHeader(&h); err != nil {
      c.JSON(http.StatusOK, err)
    }

    fmt.Printf("%#v\n", h)
    c.JSON(http.StatusOK, gin.H{"Rate": h.Rate, "Domain": h.Domain})
  })

  r.Run()

// client
// curl -H "rate:300" -H "domain:music" 127.0.0.1:8080/
// output
// {"Domain":"music","Rate":300}
}
```

### Bind HTML checkboxes

See the [detail information](https://github.com/gin-gonic/gin/issues/129#issuecomment-124260092)

main.go

```go
...

type myForm struct {
    Colors []string `form:"colors[]"`
}

...

func formHandler(c *gin.Context) {
    var fakeForm myForm
    c.ShouldBind(&fakeForm)
    c.JSON(http.StatusOK, gin.H{"color": fakeForm.Colors})
}

...

```

form.html

```html
<form action="/" method="POST">
    <p>Check some colors</p>
    <label for="red">Red</label>
    <input type="checkbox" name="colors[]" value="red" id="red">
    <label for="green">Green</label>
    <input type="checkbox" name="colors[]" value="green" id="green">
    <label for="blue">Blue</label>
    <input type="checkbox" name="colors[]" value="blue" id="blue">
    <input type="submit">
</form>
```

result:

```json
{"color":["red","green","blue"]}
```

### Multipart/Urlencoded binding

```go
type ProfileForm struct {
  Name   string                `form:"name" binding:"required"`
  Avatar *multipart.FileHeader `form:"avatar" binding:"required"`

  // or for multiple files
  // Avatars []*multipart.FileHeader `form:"avatar" binding:"required"`
}

func main() {
  router := gin.Default()
  router.POST("/profile", func(c *gin.Context) {
    // you can bind multipart form with explicit binding declaration:
    // c.ShouldBindWith(&form, binding.Form)
    // or you can simply use autobinding with ShouldBind method:
    var form ProfileForm
    // in this case proper binding will be automatically selected
    if err := c.ShouldBind(&form); err != nil {
      c.String(http.StatusBadRequest, "bad request")
      return
    }

    err := c.SaveUploadedFile(form.Avatar, form.Avatar.Filename)
    if err != nil {
      c.String(http.StatusInternalServerError, "unknown error")
      return
    }

    // db.Save(&form)

    c.String(http.StatusOK, "ok")
  })
  router.Run(":8080")
}
```

Test it with:

```sh
curl -X POST -v --form name=user --form "avatar=@./avatar.png" http://localhost:8080/profile
```

### XML, JSON, YAML, TOML and ProtoBuf rendering

```go
func main() {
  r := gin.Default()

  // gin.H is a shortcut for map[string]any
  r.GET("/someJSON", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"message": "hey", "status": http.StatusOK})
  })

  r.GET("/moreJSON", func(c *gin.Context) {
    // You can also use a struct
    var msg struct {
      Name    string `json:"user"`
      Message string
      Number  int
    }
    msg.Name = "Lena"
    msg.Message = "hey"
    msg.Number = 123
    // Note that msg.Name becomes "user" in the JSON
    // Will output  :   {"user": "Lena", "Message": "hey", "Number": 123}
    c.JSON(http.StatusOK, msg)
  })

  r.GET("/someXML", func(c *gin.Context) {
    c.XML(http.StatusOK, gin.H{"message": "hey", "status": http.StatusOK})
  })

  r.GET("/someYAML", func(c *gin.Context) {
    c.YAML(http.StatusOK, gin.H{"message": "hey", "status": http.StatusOK})
  })

  r.GET("/someTOML", func(c *gin.Context) {
    c.TOML(http.StatusOK, gin.H{"message": "hey", "status": http.StatusOK})
  })

  r.GET("/someProtoBuf", func(c *gin.Context) {
    reps := []int64{int64(1), int64(2)}
    label := "test"
    // The specific definition of protobuf is written in the testdata/protoexample file.
    data := &protoexample.Test{
      Label: &label,
      Reps:  reps,
    }
    // Note that data becomes binary data in the response
    // Will output protoexample.Test protobuf serialized data
    c.ProtoBuf(http.StatusOK, data)
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

#### SecureJSON

Using SecureJSON to prevent json hijacking. Default prepends `"while(1),"` to response body if the given struct is array values.

```go
func main() {
  r := gin.Default()

  // You can also use your own secure json prefix
  // r.SecureJsonPrefix(")]}',\n")

  r.GET("/someJSON", func(c *gin.Context) {
    names := []string{"lena", "austin", "foo"}

    // Will output  :   while(1);["lena","austin","foo"]
    c.SecureJSON(http.StatusOK, names)
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

#### JSONP

Using JSONP to request data from a server in a different domain. Add callback to response body if the query parameter callback exists.

```go
func main() {
  r := gin.Default()

  r.GET("/JSONP", func(c *gin.Context) {
    data := gin.H{
      "foo": "bar",
    }

    //callback is x
    // Will output  :   x({\"foo\":\"bar\"})
    c.JSONP(http.StatusOK, data)
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")

        // client
        // curl http://127.0.0.1:8080/JSONP?callback=x
}
```

#### AsciiJSON

Using AsciiJSON to Generates ASCII-only JSON with escaped non-ASCII characters.

```go
func main() {
  r := gin.Default()

  r.GET("/someJSON", func(c *gin.Context) {
    data := gin.H{
      "lang": "GO语言",
      "tag":  "<br>",
    }

    // will output : {"lang":"GO\u8bed\u8a00","tag":"\u003cbr\u003e"}
    c.AsciiJSON(http.StatusOK, data)
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

#### PureJSON

Normally, JSON replaces special HTML characters with their unicode entities, e.g. `<` becomes `\u003c`. If you want to encode such characters literally, you can use PureJSON instead.
This feature is unavailable in Go 1.6 and lower.

```go
func main() {
  r := gin.Default()

  // Serves unicode entities
  r.GET("/json", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
      "html": "<b>Hello, world!</b>",
    })
  })

  // Serves literal characters
  r.GET("/purejson", func(c *gin.Context) {
    c.PureJSON(http.StatusOK, gin.H{
      "html": "<b>Hello, world!</b>",
    })
  })

  // listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

### Serving static files

```go
func main() {
  router := gin.Default()
  router.Static("/assets", "./assets")
  router.StaticFS("/more_static", http.Dir("my_file_system"))
  router.StaticFile("/favicon.ico", "./resources/favicon.ico")
  router.StaticFileFS("/more_favicon.ico", "more_favicon.ico", http.Dir("my_file_system"))

  // Listen and serve on 0.0.0.0:8080
  router.Run(":8080")
}
```

### Serving data from file

```go
func main() {
  router := gin.Default()

  router.GET("/local/file", func(c *gin.Context) {
    c.File("local/file.go")
  })

  var fs http.FileSystem = // ...
  router.GET("/fs/file", func(c *gin.Context) {
    c.FileFromFS("fs/file.go", fs)
  })
}

```

### Serving data from reader

```go
func main() {
  router := gin.Default()
  router.GET("/someDataFromReader", func(c *gin.Context) {
    response, err := http.Get("https://raw.githubusercontent.com/gin-gonic/logo/master/color.png")
    if err != nil || response.StatusCode != http.StatusOK {
      c.Status(http.StatusServiceUnavailable)
      return
    }

    reader := response.Body
     defer reader.Close()
    contentLength := response.ContentLength
    contentType := response.Header.Get("Content-Type")

    extraHeaders := map[string]string{
      "Content-Disposition": `attachment; filename="gopher.png"`,
    }

    c.DataFromReader(http.StatusOK, contentLength, contentType, reader, extraHeaders)
  })
  router.Run(":8080")
}
```

### HTML rendering

Using LoadHTMLGlob() or LoadHTMLFiles() or LoadHTMLFS()

```go
//go:embed templates/*
var templates embed.FS

func main() {
  router := gin.Default()
  router.LoadHTMLGlob("templates/*")
  //router.LoadHTMLFiles("templates/template1.html", "templates/template2.html")
  //router.LoadHTMLFS(http.Dir("templates"), "template1.html", "template2.html")
  //or
  //router.LoadHTMLFS(http.FS(templates), "templates/template1.html", "templates/template2.html")
  router.GET("/index", func(c *gin.Context) {
    c.HTML(http.StatusOK, "index.tmpl", gin.H{
      "title": "Main website",
    })
  })
  router.Run(":8080")
}
```

templates/index.tmpl

```html
<html>
  <h1>
    {{ .title }}
  </h1>
</html>
```

Using templates with same name in different directories

```go
func main() {
  router := gin.Default()
  router.LoadHTMLGlob("templates/**/*")
  router.GET("/posts/index", func(c *gin.Context) {
    c.HTML(http.StatusOK, "posts/index.tmpl", gin.H{
      "title": "Posts",
    })
  })
  router.GET("/users/index", func(c *gin.Context) {
    c.HTML(http.StatusOK, "users/index.tmpl", gin.H{
      "title": "Users",
    })
  })
  router.Run(":8080")
}
```

templates/posts/index.tmpl

```html
{{ define "posts/index.tmpl" }}
<html><h1>
  {{ .title }}
</h1>
<p>Using posts/index.tmpl</p>
</html>
{{ end }}
```

templates/users/index.tmpl

```html
{{ define "users/index.tmpl" }}
<html><h1>
  {{ .title }}
</h1>
<p>Using users/index.tmpl</p>
</html>
{{ end }}
```

#### Custom Template renderer

You can also use your own html template render

```go
import "html/template"

func main() {
  router := gin.Default()
  html := template.Must(template.ParseFiles("file1", "file2"))
  router.SetHTMLTemplate(html)
  router.Run(":8080")
}
```

#### Custom Delimiters

You may use custom delims

```go
  r := gin.Default()
  r.Delims("{[{", "}]}")
  r.LoadHTMLGlob("/path/to/templates")
```

#### Custom Template Funcs

See the detailed [example code](https://github.com/gin-gonic/examples/tree/master/template).

main.go

```go
import (
  "fmt"
  "html/template"
  "net/http"
  "time"

  "github.com/gin-gonic/gin"
)

func formatAsDate(t time.Time) string {
  year, month, day := t.Date()
  return fmt.Sprintf("%d/%02d/%02d", year, month, day)
}

func main() {
  router := gin.Default()
  router.Delims("{[{", "}]}")
  router.SetFuncMap(template.FuncMap{
      "formatAsDate": formatAsDate,
  })
  router.LoadHTMLFiles("./testdata/template/raw.tmpl")

  router.GET("/raw", func(c *gin.Context) {
      c.HTML(http.StatusOK, "raw.tmpl", gin.H{
          "now": time.Date(2017, 07, 01, 0, 0, 0, 0, time.UTC),
      })
  })

  router.Run(":8080")
}

```

raw.tmpl

```html
Date: {[{.now | formatAsDate}]}
```

Result:

```sh
Date: 2017/07/01
```

### Multitemplate

Gin allows only one html.Template by default. Check [a multitemplate render](https://github.com/gin-contrib/multitemplate) for using features like go 1.6 `block template`.

### Redirects

Issuing a HTTP redirect is easy. Both internal and external locations are supported.

```go
r.GET("/test", func(c *gin.Context) {
  c.Redirect(http.StatusMovedPermanently, "http://www.google.com/")
})
```

Issuing a HTTP redirect from POST. Refer to issue: [#444](https://github.com/gin-gonic/gin/issues/444)

```go
r.POST("/test", func(c *gin.Context) {
  c.Redirect(http.StatusFound, "/foo")
})
```

Issuing a Router redirect, use `HandleContext` like below.

``` go
r.GET("/test", func(c *gin.Context) {
    c.Request.URL.Path = "/test2"
    r.HandleContext(c)
})
r.GET("/test2", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"hello": "world"})
})
```

### Custom Middleware

```go
func Logger() gin.HandlerFunc {
  return func(c *gin.Context) {
    t := time.Now()

    // Set example variable
    c.Set("example", "12345")

    // before request

    c.Next()

    // after request
    latency := time.Since(t)
    log.Print(latency)

    // access the status we are sending
    status := c.Writer.Status()
    log.Println(status)
  }
}

func main() {
  r := gin.New()
  r.Use(Logger())

  r.GET("/test", func(c *gin.Context) {
    example := c.MustGet("example").(string)

    // it would print: "12345"
    log.Println(example)
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

### Using BasicAuth() middleware

```go
// simulate some private data
var secrets = gin.H{
  "foo":    gin.H{"email": "foo@bar.com", "phone": "123433"},
  "austin": gin.H{"email": "austin@example.com", "phone": "666"},
  "lena":   gin.H{"email": "lena@guapa.com", "phone": "523443"},
}

func main() {
  r := gin.Default()

  // Group using gin.BasicAuth() middleware
  // gin.Accounts is a shortcut for map[string]string
  authorized := r.Group("/admin", gin.BasicAuth(gin.Accounts{
    "foo":    "bar",
    "austin": "1234",
    "lena":   "hello2",
    "manu":   "4321",
  }))

  // /admin/secrets endpoint
  // hit "localhost:8080/admin/secrets
  authorized.GET("/secrets", func(c *gin.Context) {
    // get user, it was set by the BasicAuth middleware
    user := c.MustGet(gin.AuthUserKey).(string)
    if secret, ok := secrets[user]; ok {
      c.JSON(http.StatusOK, gin.H{"user": user, "secret": secret})
    } else {
      c.JSON(http.StatusOK, gin.H{"user": user, "secret": "NO SECRET :("})
    }
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

### Goroutines inside a middleware

When starting new Goroutines inside a middleware or handler, you **SHOULD NOT** use the original context inside it, you have to use a read-only copy.

```go
func main() {
  r := gin.Default()

  r.GET("/long_async", func(c *gin.Context) {
    // create copy to be used inside the goroutine
    cCp := c.Copy()
    go func() {
      // simulate a long task with time.Sleep(). 5 seconds
      time.Sleep(5 * time.Second)

      // note that you are using the copied context "cCp", IMPORTANT
      log.Println("Done! in path " + cCp.Request.URL.Path)
    }()
  })

  r.GET("/long_sync", func(c *gin.Context) {
    // simulate a long task with time.Sleep(). 5 seconds
    time.Sleep(5 * time.Second)

    // since we are NOT using a goroutine, we do not have to copy the context
    log.Println("Done! in path " + c.Request.URL.Path)
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

### Custom HTTP configuration

Use `http.ListenAndServe()` directly, like this:

```go
func main() {
  router := gin.Default()
  http.ListenAndServe(":8080", router)
}
```

or

```go
func main() {
  router := gin.Default()

  s := &http.Server{
    Addr:           ":8080",
    Handler:        router,
    ReadTimeout:    10 * time.Second,
    WriteTimeout:   10 * time.Second,
    MaxHeaderBytes: 1 << 20,
  }
  s.ListenAndServe()
}
```

### Support Let's Encrypt

example for 1-line LetsEncrypt HTTPS servers.

```go
package main

import (
  "log"
  "net/http"

  "github.com/gin-gonic/autotls"
  "github.com/gin-gonic/gin"
)

func main() {
  r := gin.Default()

  // Ping handler
  r.GET("/ping", func(c *gin.Context) {
    c.String(http.StatusOK, "pong")
  })

  log.Fatal(autotls.Run(r, "example1.com", "example2.com"))
}
```

example for custom autocert manager.

```go
package main

import (
  "log"
  "net/http"

  "github.com/gin-gonic/autotls"
  "github.com/gin-gonic/gin"
  "golang.org/x/crypto/acme/autocert"
)

func main() {
  r := gin.Default()

  // Ping handler
  r.GET("/ping", func(c *gin.Context) {
    c.String(http.StatusOK, "pong")
  })

  m := autocert.Manager{
    Prompt:     autocert.AcceptTOS,
    HostPolicy: autocert.HostWhitelist("example1.com", "example2.com"),
    Cache:      autocert.DirCache("/var/www/.cache"),
  }

  log.Fatal(autotls.RunWithManager(r, &m))
}
```

### Run multiple service using Gin

See the [question](https://github.com/gin-gonic/gin/issues/346) and try the following example:

```go
package main

import (
  "log"
  "net/http"
  "time"

  "github.com/gin-gonic/gin"
  "golang.org/x/sync/errgroup"
)

var (
  g errgroup.Group
)

func router01() http.Handler {
  e := gin.New()
  e.Use(gin.Recovery())
  e.GET("/", func(c *gin.Context) {
    c.JSON(
      http.StatusOK,
      gin.H{
        "code":  http.StatusOK,
        "error": "Welcome server 01",
      },
    )
  })

  return e
}

func router02() http.Handler {
  e := gin.New()
  e.Use(gin.Recovery())
  e.GET("/", func(c *gin.Context) {
    c.JSON(
      http.StatusOK,
      gin.H{
        "code":  http.StatusOK,
        "error": "Welcome server 02",
      },
    )
  })

  return e
}

func main() {
  server01 := &http.Server{
    Addr:         ":8080",
    Handler:      router01(),
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
  }

  server02 := &http.Server{
    Addr:         ":8081",
    Handler:      router02(),
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
  }

  g.Go(func() error {
    err := server01.ListenAndServe()
    if err != nil && err != http.ErrServerClosed {
      log.Fatal(err)
    }
    return err
  })

  g.Go(func() error {
    err := server02.ListenAndServe()
    if err != nil && err != http.ErrServerClosed {
      log.Fatal(err)
    }
    return err
  })

  if err := g.Wait(); err != nil {
    log.Fatal(err)
  }
}
```

### Graceful shutdown or restart

There are a few approaches you can use to perform a graceful shutdown or restart. You can make use of third-party packages specifically built for that, or you can manually do the same with the functions and methods from the built-in packages.

#### Third-party packages

We can use [fvbock/endless](https://github.com/fvbock/endless) to replace the default `ListenAndServe`. Refer to issue [#296](https://github.com/gin-gonic/gin/issues/296) for more details.

```go
router := gin.Default()
router.GET("/", handler)
// [...]
endless.ListenAndServe(":4242", router)
```

Alternatives:

* [grace](https://github.com/facebookgo/grace): Graceful restart & zero downtime deploy for Go servers.
* [graceful](https://github.com/tylerb/graceful): Graceful is a Go package enabling graceful shutdown of an http.Handler server.
* [manners](https://github.com/braintree/manners): A polite Go HTTP server that shuts down gracefully.

#### Manually

In case you are using Go 1.8 or a later version, you may not need to use those libraries. Consider using `http.Server`'s built-in [Shutdown()](https://pkg.go.dev/net/http#Server.Shutdown) method for graceful shutdowns. The example below describes its usage, and we've got more examples using gin [here](https://github.com/gin-gonic/examples/tree/master/graceful-shutdown).

```go
// +build go1.8

package main

import (
  "context"
  "log"
  "net/http"
  "os"
  "os/signal"
  "syscall"
  "time"

  "github.com/gin-gonic/gin"
)

func main() {
  router := gin.Default()
  router.GET("/", func(c *gin.Context) {
    time.Sleep(5 * time.Second)
    c.String(http.StatusOK, "Welcome Gin Server")
  })

  srv := &http.Server{
    Addr:    ":8080",
    Handler: router,
  }

  // Initializing the server in a goroutine so that
  // it won't block the graceful shutdown handling below
  go func() {
    if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
      log.Printf("listen: %s\n", err)
    }
  }()

  // Wait for interrupt signal to gracefully shutdown the server with
  // a timeout of 5 seconds.
  quit := make(chan os.Signal)
  // kill (no param) default send syscall.SIGTERM
  // kill -2 is syscall.SIGINT
  // kill -9 is syscall.SIGKILL but can't be caught, so don't need to add it
  signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
  <-quit
  log.Println("Shutting down server...")

  // The context is used to inform the server it has 5 seconds to finish
  // the request it is currently handling
  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
  defer cancel()

  if err := srv.Shutdown(ctx); err != nil {
    log.Fatal("Server forced to shutdown:", err)
  }

  log.Println("Server exiting")
}
```

### Build a single binary with templates

You can build a server into a single binary containing templates by using the [embed](https://pkg.go.dev/embed) package.

```go
package main

import (
  "embed"
  "html/template"
  "net/http"

  "github.com/gin-gonic/gin"
)

//go:embed assets/* templates/*
var f embed.FS

func main() {
  router := gin.Default()
  templ := template.Must(template.New("").ParseFS(f, "templates/*.tmpl", "templates/foo/*.tmpl"))
  router.SetHTMLTemplate(templ)

  // example: /public/assets/images/example.png
  router.StaticFS("/public", http.FS(f))

  router.GET("/", func(c *gin.Context) {
    c.HTML(http.StatusOK, "index.tmpl", gin.H{
      "title": "Main website",
    })
  })

  router.GET("/foo", func(c *gin.Context) {
    c.HTML(http.StatusOK, "bar.tmpl", gin.H{
      "title": "Foo website",
    })
  })

  router.GET("favicon.ico", func(c *gin.Context) {
    file, _ := f.ReadFile("assets/favicon.ico")
    c.Data(
      http.StatusOK,
      "image/x-icon",
      file,
    )
  })

  router.Run(":8080")
}
```

See a complete example in the `https://github.com/gin-gonic/examples/tree/master/assets-in-binary/example02` directory.

### Bind form-data request with custom struct

The follow example using custom struct:

```go
type StructA struct {
    FieldA string `form:"field_a"`
}

type StructB struct {
    NestedStruct StructA
    FieldB string `form:"field_b"`
}

type StructC struct {
    NestedStructPointer *StructA
    FieldC string `form:"field_c"`
}

type StructD struct {
    NestedAnonyStruct struct {
        FieldX string `form:"field_x"`
    }
    FieldD string `form:"field_d"`
}

func GetDataB(c *gin.Context) {
    var b StructB
    c.Bind(&b)
    c.JSON(http.StatusOK, gin.H{
        "a": b.NestedStruct,
        "b": b.FieldB,
    })
}

func GetDataC(c *gin.Context) {
    var b StructC
    c.Bind(&b)
    c.JSON(http.StatusOK, gin.H{
        "a": b.NestedStructPointer,
        "c": b.FieldC,
    })
}

func GetDataD(c *gin.Context) {
    var b StructD
    c.Bind(&b)
    c.JSON(http.StatusOK, gin.H{
        "x": b.NestedAnonyStruct,
        "d": b.FieldD,
    })
}

func main() {
    r := gin.Default()
    r.GET("/getb", GetDataB)
    r.GET("/getc", GetDataC)
    r.GET("/getd", GetDataD)

    r.Run()
}
```

Using the command `curl` command result:

```sh
$ curl "http://localhost:8080/getb?field_a=hello&field_b=world"
{"a":{"FieldA":"hello"},"b":"world"}
$ curl "http://localhost:8080/getc?field_a=hello&field_c=world"
{"a":{"FieldA":"hello"},"c":"world"}
$ curl "http://localhost:8080/getd?field_x=hello&field_d=world"
{"d":"world","x":{"FieldX":"hello"}}
```

### Try to bind body into different structs

The normal methods for binding request body consumes `c.Request.Body` and they
cannot be called multiple times.

```go
type formA struct {
  Foo string `json:"foo" xml:"foo" binding:"required"`
}

type formB struct {
  Bar string `json:"bar" xml:"bar" binding:"required"`
}

func SomeHandler(c *gin.Context) {
  objA := formA{}
  objB := formB{}
  // Calling c.ShouldBind consumes c.Request.Body and it cannot be reused.
  if errA := c.ShouldBind(&objA); errA == nil {
    c.String(http.StatusOK, `the body should be formA`)
  // Always an error is occurred by this because c.Request.Body is EOF now.
  } else if errB := c.ShouldBind(&objB); errB == nil {
    c.String(http.StatusOK, `the body should be formB`)
  } else {
    ...
  }
}
```

For this, you can use `c.ShouldBindBodyWith` or shortcuts.

- `c.ShouldBindBodyWithJSON` is a shortcut for c.ShouldBindBodyWith(obj, binding.JSON).
- `c.ShouldBindBodyWithXML` is a shortcut for c.ShouldBindBodyWith(obj, binding.XML).
- `c.ShouldBindBodyWithYAML` is a shortcut for c.ShouldBindBodyWith(obj, binding.YAML).
- `c.ShouldBindBodyWithTOML` is a shortcut for c.ShouldBindBodyWith(obj, binding.TOML).

```go
func SomeHandler(c *gin.Context) {
  objA := formA{}
  objB := formB{}
  // This reads c.Request.Body and stores the result into the context.
  if errA := c.ShouldBindBodyWith(&objA, binding.Form); errA == nil {
    c.String(http.StatusOK, `the body should be formA`)
  // At this time, it reuses body stored in the context.
  } else if errB := c.ShouldBindBodyWith(&objB, binding.JSON); errB == nil {
    c.String(http.StatusOK, `the body should be formB JSON`)
  // And it can accepts other formats
  } else if errB2 := c.ShouldBindBodyWithXML(&objB); errB2 == nil {
    c.String(http.StatusOK, `the body should be formB XML`)
  } else {
    ...
  }
}
```

1. `c.ShouldBindBodyWith` stores body into the context before binding. This has
a slight impact to performance, so you should not use this method if you are
enough to call binding at once.
2. This feature is only needed for some formats -- `JSON`, `XML`, `MsgPack`,
`ProtoBuf`. For other formats, `Query`, `Form`, `FormPost`, `FormMultipart`,
can be called by `c.ShouldBind()` multiple times without any damage to
performance (See [#1341](https://github.com/gin-gonic/gin/pull/1341)).

### Bind form-data request with custom struct and custom tag

```go
const (
  customerTag = "url"
  defaultMemory = 32 << 20
)

type customerBinding struct {}

func (customerBinding) Name() string {
  return "form"
}

func (customerBinding) Bind(req *http.Request, obj any) error {
  if err := req.ParseForm(); err != nil {
    return err
  }
  if err := req.ParseMultipartForm(defaultMemory); err != nil {
    if err != http.ErrNotMultipart {
      return err
    }
  }
  if err := binding.MapFormWithTag(obj, req.Form, customerTag); err != nil {
    return err
  }
  return validate(obj)
}

func validate(obj any) error {
  if binding.Validator == nil {
    return nil
  }
  return binding.Validator.ValidateStruct(obj)
}

// Now we can do this!!!
// FormA is an external type that we can't modify it's tag
type FormA struct {
  FieldA string `url:"field_a"`
}

func ListHandler(s *Service) func(ctx *gin.Context) {
  return func(ctx *gin.Context) {
    var urlBinding = customerBinding{}
    var opt FormA
    err := ctx.MustBindWith(&opt, urlBinding)
    if err != nil {
      ...
    }
    ...
  }
}
```

### http2 server push

http.Pusher is supported only **go1.8+**. See the [golang blog](https://go.dev/blog/h2push) for detail information.

```go
package main

import (
  "html/template"
  "log"
  "net/http"

  "github.com/gin-gonic/gin"
)

var html = template.Must(template.New("https").Parse(`
<html>
<head>
  <title>Https Test</title>
  <script src="/assets/app.js"></script>
</head>
<body>
  <h1 style="color:red;">Welcome, Ginner!</h1>
</body>
</html>
`))

func main() {
  r := gin.Default()
  r.Static("/assets", "./assets")
  r.SetHTMLTemplate(html)

  r.GET("/", func(c *gin.Context) {
    if pusher := c.Writer.Pusher(); pusher != nil {
      // use pusher.Push() to do server push
      if err := pusher.Push("/assets/app.js", nil); err != nil {
        log.Printf("Failed to push: %v", err)
      }
    }
    c.HTML(http.StatusOK, "https", gin.H{
      "status": "success",
    })
  })

  // Listen and Server in https://127.0.0.1:8080
  r.RunTLS(":8080", "./testdata/server.pem", "./testdata/server.key")
}
```

### Define format for the log of routes

The default log of routes is:

```sh
[GIN-debug] POST   /foo                      --> main.main.func1 (3 handlers)
[GIN-debug] GET    /bar                      --> main.main.func2 (3 handlers)
[GIN-debug] GET    /status                   --> main.main.func3 (3 handlers)
```

If you want to log this information in given format (e.g. JSON, key values or something else), then you can define this format with `gin.DebugPrintRouteFunc`.
In the example below, we log all routes with standard log package but you can use another log tools that suits of your needs.

```go
import (
  "log"
  "net/http"

  "github.com/gin-gonic/gin"
)

func main() {
  r := gin.Default()
  gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
    log.Printf("endpoint %v %v %v %v\n", httpMethod, absolutePath, handlerName, nuHandlers)
  }

  r.POST("/foo", func(c *gin.Context) {
    c.JSON(http.StatusOK, "foo")
  })

  r.GET("/bar", func(c *gin.Context) {
    c.JSON(http.StatusOK, "bar")
  })

  r.GET("/status", func(c *gin.Context) {
    c.JSON(http.StatusOK, "ok")
  })

  // Listen and Server in http://0.0.0.0:8080
  r.Run()
}
```

### Set and get a cookie

```go
import (
  "fmt"

  "github.com/gin-gonic/gin"
)

func main() {
  router := gin.Default()

  router.GET("/cookie", func(c *gin.Context) {
    cookie, err := c.Cookie("gin_cookie")

    if err != nil {
      cookie = "NotSet"
      // Using http.Cookie struct for more control
      c.SetCookieData(&http.Cookie{
        Name:       "gin_cookie",
        Value:      "test",
        Path:       "/",
        Domain:     "localhost",
        MaxAge:     3600,
        Secure:     false,
        HttpOnly:   true,
        // Additional fields available in http.Cookie
        Expires:    time.Now().Add(24 * time.Hour),
        // Partitioned: true, // Available in newer Go versions
      })
    }

    fmt.Printf("Cookie value: %s \n", cookie)
  })

  router.Run()
}
```

You can also use the `SetCookieData` method, which accepts a `*http.Cookie` directly for more flexibility:

```go
import (
  "fmt"
  "net/http"
  "time"

  "github.com/gin-gonic/gin"
)

func main() {
  router := gin.Default()

  router.GET("/cookie", func(c *gin.Context) {
      cookie, err := c.Cookie("gin_cookie")

      if err != nil {
          cookie = "NotSet"
          // Using http.Cookie struct for more control
          c.SetCookieData(&http.Cookie{
              Name:       "gin_cookie",
              Value:      "test",
              Path:       "/",
              Domain:     "localhost",
              MaxAge:     3600,
              Secure:     false,
              HttpOnly:   true,
              // Additional fields available in http.Cookie
              Expires:    time.Now().Add(24 * time.Hour),
              // Partitioned: true, // Available in newer Go versions
          })
      }

      fmt.Printf("Cookie value: %s \n", cookie)
  })

  router.Run()
}
```

### Custom json codec at runtime

Gin support custom json serialization and deserialization logic without using compile tags.

1. Define a custom struct implements the `json.Core` interface.

2. Before your engine starts, assign values to `json.API` using the custom struct.

```go
package main

import (
  "io"

  "github.com/gin-gonic/gin"
  "github.com/gin-gonic/gin/codec/json"
  jsoniter "github.com/json-iterator/go"
)

var customConfig = jsoniter.Config{
  EscapeHTML:             true,
  SortMapKeys:            true,
  ValidateJsonRawMessage: true,
}.Froze()

// implement api.JsonApi
type customJsonApi struct {
}

func (j customJsonApi) Marshal(v any) ([]byte, error) {
  return customConfig.Marshal(v)
}

func (j customJsonApi) Unmarshal(data []byte, v any) error {
  return customConfig.Unmarshal(data, v)
}

func (j customJsonApi) MarshalIndent(v any, prefix, indent string) ([]byte, error) {
  return customConfig.MarshalIndent(v, prefix, indent)
}

func (j customJsonApi) NewEncoder(writer io.Writer) json.Encoder {
  return customConfig.NewEncoder(writer)
}

func (j customJsonApi) NewDecoder(reader io.Reader) json.Decoder {
  return customConfig.NewDecoder(reader)
}

func main() {
  //Replace the default json api
  json.API = customJsonApi{}

  //Start your gin engine
  router := gin.Default()
  router.Run(":8080")
}
```

## Don't trust all proxies

Gin lets you specify which headers to hold the real client IP (if any),
as well as specifying which proxies (or direct clients) you trust to
specify one of these headers.

Use function `SetTrustedProxies()` on your `gin.Engine` to specify network addresses
or network CIDRs from where clients which their request headers related to client
IP can be trusted. They can be IPv4 addresses, IPv4 CIDRs, IPv6 addresses or
IPv6 CIDRs.

**Attention:** Gin trusts all proxies by default if you don't specify a trusted
proxy using the function above, **this is NOT safe**. At the same time, if you don't
use any proxy, you can disable this feature by using `Engine.SetTrustedProxies(nil)`,
then `Context.ClientIP()` will return the remote address directly to avoid some
unnecessary computation.

```go
import (
  "fmt"

  "github.com/gin-gonic/gin"
)

func main() {
  router := gin.Default()
  router.SetTrustedProxies([]string{"192.168.1.2"})

  router.GET("/", func(c *gin.Context) {
    // If the client is 192.168.1.2, use the X-Forwarded-For
    // header to deduce the original client IP from the trust-
    // worthy parts of that header.
    // Otherwise, simply return the direct client IP
    fmt.Printf("ClientIP: %s\n", c.ClientIP())
  })
  router.Run()
}
```

**Notice:** If you are using a CDN service, you can set the `Engine.TrustedPlatform`
to skip TrustedProxies check, it has a higher priority than TrustedProxies.
Look at the example below:

```go
import (
  "fmt"

  "github.com/gin-gonic/gin"
)

func main() {
  router := gin.Default()
  // Use predefined header gin.PlatformXXX
  // Google App Engine
  router.TrustedPlatform = gin.PlatformGoogleAppEngine
  // Cloudflare
  router.TrustedPlatform = gin.PlatformCloudflare
  // Fly.io
  router.TrustedPlatform = gin.PlatformFlyIO
  // Or, you can set your own trusted request header. But be sure your CDN
  // prevents users from passing this header! For example, if your CDN puts
  // the client IP in X-CDN-Client-IP:
  router.TrustedPlatform = "X-CDN-Client-IP"

  router.GET("/", func(c *gin.Context) {
    // If you set TrustedPlatform, ClientIP() will resolve the
    // corresponding header and return IP directly
    fmt.Printf("ClientIP: %s\n", c.ClientIP())
  })
  router.Run()
}
```

## Testing

The `net/http/httptest` package is preferable way for HTTP testing.

```go
package main

import (
  "net/http"

  "github.com/gin-gonic/gin"
)

func setupRouter() *gin.Engine {
  r := gin.Default()
  r.GET("/ping", func(c *gin.Context) {
    c.String(http.StatusOK, "pong")
  })
  return r
}

func main() {
  r := setupRouter()
  r.Run(":8080")
}
```

Test for code example above:

```go
package main

import (
  "net/http"
  "net/http/httptest"
  "testing"

  "github.com/stretchr/testify/assert"
)

func TestPingRoute(t *testing.T) {
  router := setupRouter()

  w := httptest.NewRecorder()
  req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
  router.ServeHTTP(w, req)

  assert.Equal(t, http.StatusOK, w.Code)
  assert.Equal(t, "pong", w.Body.String())
}
```

## README.md

# Gin Web Framework

<img align="right" width="159px" src="https://raw.githubusercontent.com/gin-gonic/logo/master/color.png">

[![Build Status](https://github.com/gin-gonic/gin/actions/workflows/gin.yml/badge.svg?branch=master)](https://github.com/gin-gonic/gin/actions/workflows/gin.yml)
[![codecov](https://codecov.io/gh/gin-gonic/gin/branch/master/graph/badge.svg)](https://codecov.io/gh/gin-gonic/gin)
[![Go Report Card](https://goreportcard.com/badge/github.com/gin-gonic/gin)](https://goreportcard.com/report/github.com/gin-gonic/gin)
[![Go Reference](https://pkg.go.dev/badge/github.com/gin-gonic/gin?status.svg)](https://pkg.go.dev/github.com/gin-gonic/gin?tab=doc)
[![Sourcegraph](https://sourcegraph.com/github.com/gin-gonic/gin/-/badge.svg)](https://sourcegraph.com/github.com/gin-gonic/gin?badge)
[![Open Source Helpers](https://www.codetriage.com/gin-gonic/gin/badges/users.svg)](https://www.codetriage.com/gin-gonic/gin)
[![Release](https://img.shields.io/github/release/gin-gonic/gin.svg?style=flat-square)](https://github.com/gin-gonic/gin/releases)
[![TODOs](https://badgen.net/https/api.tickgit.com/badgen/github.com/gin-gonic/gin)](https://www.tickgit.com/browse?repo=github.com/gin-gonic/gin)

Gin is a web framework written in [Go](https://go.dev/). It features a martini-like API with performance that is up to 40 times faster thanks to [httprouter](https://github.com/julienschmidt/httprouter).
If you need performance and good productivity, you will love Gin.

**Gin's key features are:**

- Zero allocation router
- Speed
- Middleware support
- Crash-free
- JSON validation
- Route grouping
- Error management
- Built-in rendering
- Extensible

## Getting started

### Prerequisites

Gin requires [Go](https://go.dev/) version [1.23](https://go.dev/doc/devel/release#go1.23.0) or above.

### Getting Gin

With [Go's module support](https://go.dev/wiki/Modules#how-to-use-modules), `go [build|run|test]` automatically fetches the necessary dependencies when you add the import in your code:

```sh
import "github.com/gin-gonic/gin"
```

Alternatively, use `go get`:

```sh
go get -u github.com/gin-gonic/gin
```

### Running Gin

A basic example:

```go
package main

import (
  "net/http"

  "github.com/gin-gonic/gin"
)

func main() {
  r := gin.Default()
  r.GET("/ping", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
      "message": "pong",
    })
  })
  r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
```

To run the code, use the `go run` command, like:

```sh
go run example.go
```

Then visit [`0.0.0.0:8080/ping`](http://0.0.0.0:8080/ping) in your browser to see the response!

### See more examples

#### Quick Start

Learn and practice with the [Gin Quick Start](docs/doc.md), which includes API examples and builds tag.

#### Examples

A number of ready-to-run examples demonstrating various use cases of Gin are available in the [Gin examples](https://github.com/gin-gonic/examples) repository.

## Documentation

See the [API documentation on go.dev](https://pkg.go.dev/github.com/gin-gonic/gin).

The documentation is also available on [gin-gonic.com](https://gin-gonic.com) in several languages:

- [English](https://gin-gonic.com/en/docs/)
- [简体中文](https://gin-gonic.com/zh-cn/docs/)
- [繁體中文](https://gin-gonic.com/zh-tw/docs/)
- [日本語](https://gin-gonic.com/ja/docs/)
- [Español](https://gin-gonic.com/es/docs/)
- [한국어](https://gin-gonic.com/ko-kr/docs/)
- [Turkish](https://gin-gonic.com/tr/docs/)
- [Persian](https://gin-gonic.com/fa/docs/)
- [Português](https://gin-gonic.com/pt/docs/)
- [Russian](https://gin-gonic.com/ru/docs/)
- [Indonesian](https://gin-gonic.com/id/docs/)

### Articles

- [Tutorial: Developing a RESTful API with Go and Gin](https://go.dev/doc/tutorial/web-service-gin)

## Benchmarks

Gin uses a custom version of [HttpRouter](https://github.com/julienschmidt/httprouter), [see all benchmarks](/BENCHMARKS.md).

| Benchmark name                 |       (1) |             (2) |          (3) |             (4) |
| ------------------------------ | --------: | --------------: | -----------: | --------------: |
| BenchmarkGin_GithubAll         | **43550** | **27364 ns/op** |   **0 B/op** | **0 allocs/op** |
| BenchmarkAce_GithubAll         |     40543 |     29670 ns/op |       0 B/op |     0 allocs/op |
| BenchmarkAero_GithubAll        |     57632 |     20648 ns/op |       0 B/op |     0 allocs/op |
| BenchmarkBear_GithubAll        |      9234 |    216179 ns/op |   86448 B/op |   943 allocs/op |
| BenchmarkBeego_GithubAll       |      7407 |    243496 ns/op |   71456 B/op |   609 allocs/op |
| BenchmarkBone_GithubAll        |       420 |   2922835 ns/op |  720160 B/op |  8620 allocs/op |
| BenchmarkChi_GithubAll         |      7620 |    238331 ns/op |   87696 B/op |   609 allocs/op |
| BenchmarkDenco_GithubAll       |     18355 |     64494 ns/op |   20224 B/op |   167 allocs/op |
| BenchmarkEcho_GithubAll        |     31251 |     38479 ns/op |       0 B/op |     0 allocs/op |
| BenchmarkGocraftWeb_GithubAll  |      4117 |    300062 ns/op |  131656 B/op |  1686 allocs/op |
| BenchmarkGoji_GithubAll        |      3274 |    416158 ns/op |   56112 B/op |   334 allocs/op |
| BenchmarkGojiv2_GithubAll      |      1402 |    870518 ns/op |  352720 B/op |  4321 allocs/op |
| BenchmarkGoJsonRest_GithubAll  |      2976 |    401507 ns/op |  134371 B/op |  2737 allocs/op |
| BenchmarkGoRestful_GithubAll   |       410 |   2913158 ns/op |  910144 B/op |  2938 allocs/op |
| BenchmarkGorillaMux_GithubAll  |       346 |   3384987 ns/op |  251650 B/op |  1994 allocs/op |
| BenchmarkGowwwRouter_GithubAll |     10000 |    143025 ns/op |   72144 B/op |   501 allocs/op |
| BenchmarkHttpRouter_GithubAll  |     55938 |     21360 ns/op |       0 B/op |     0 allocs/op |
| BenchmarkHttpTreeMux_GithubAll |     10000 |    153944 ns/op |   65856 B/op |   671 allocs/op |
| BenchmarkKocha_GithubAll       |     10000 |    106315 ns/op |   23304 B/op |   843 allocs/op |
| BenchmarkLARS_GithubAll        |     47779 |     25084 ns/op |       0 B/op |     0 allocs/op |
| BenchmarkMacaron_GithubAll     |      3266 |    371907 ns/op |  149409 B/op |  1624 allocs/op |
| BenchmarkMartini_GithubAll     |       331 |   3444706 ns/op |  226551 B/op |  2325 allocs/op |
| BenchmarkPat_GithubAll         |       273 |   4381818 ns/op | 1483152 B/op | 26963 allocs/op |
| BenchmarkPossum_GithubAll      |     10000 |    164367 ns/op |   84448 B/op |   609 allocs/op |
| BenchmarkR2router_GithubAll    |     10000 |    160220 ns/op |   77328 B/op |   979 allocs/op |
| BenchmarkRivet_GithubAll       |     14625 |     82453 ns/op |   16272 B/op |   167 allocs/op |
| BenchmarkTango_GithubAll       |      6255 |    279611 ns/op |   63826 B/op |  1618 allocs/op |
| BenchmarkTigerTonic_GithubAll  |      2008 |    687874 ns/op |  193856 B/op |  4474 allocs/op |
| BenchmarkTraffic_GithubAll     |       355 |   3478508 ns/op |  820744 B/op | 14114 allocs/op |
| BenchmarkVulcan_GithubAll      |      6885 |    193333 ns/op |   19894 B/op |   609 allocs/op |

- (1): Total Repetitions achieved in constant time, higher means more confident result
- (2): Single Repetition Duration (ns/op), lower is better
- (3): Heap Memory (B/op), lower is better
- (4): Average Allocations per Repetition (allocs/op), lower is better

## Middleware

You can find many useful Gin middlewares at [gin-contrib](https://github.com/gin-contrib) and [gin-gonic/contrib](https://github.com/gin-gonic/contrib).

## Uses

Here are some awesome projects that are using the [Gin](https://github.com/gin-gonic/gin) web framework.

- [gorush](https://github.com/appleboy/gorush): A push notification server.
- [fnproject](https://github.com/fnproject/fn): A container native, cloud agnostic serverless platform.
- [photoprism](https://github.com/photoprism/photoprism): Personal photo management powered by Google TensorFlow.
- [lura](https://github.com/luraproject/lura): Ultra performant API Gateway with middleware.
- [picfit](https://github.com/thoas/picfit): An image resizing server.
- [dkron](https://github.com/distribworks/dkron): Distributed, fault tolerant job scheduling system.

## Contributing

Gin is the work of hundreds of contributors. We appreciate your help!

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details on submitting patches and the contribution workflow.

## doc.md

# Gin Quick Start

## Contents

- [Build Tags](#build-tags)
  - [Build with json replacement](#build-with-json-replacement)
  - [Build without `MsgPack` rendering feature](#build-without-msgpack-rendering-feature)
- [API Examples](#api-examples)
  - [Using GET, POST, PUT, PATCH, DELETE and OPTIONS](#using-get-post-put-patch-delete-and-options)
  - [Parameters in path](#parameters-in-path)
  - [Querystring parameters](#querystring-parameters)
  - [Multipart/Urlencoded Form](#multiparturlencoded-form)
  - [Another example: query + post form](#another-example-query--post-form)
  - [Map as querystring or postform parameters](#map-as-querystring-or-postform-parameters)
  - [Upload files](#upload-files)
    - [Single file](#single-file)
    - [Multiple files](#multiple-files)
  - [Grouping routes](#grouping-routes)
  - [Blank Gin without middleware by default](#blank-gin-without-middleware-by-default)
  - [Using middleware](#using-middleware)
  - [Custom Recovery behavior](#custom-recovery-behavior)
  - [How to write log file](#how-to-write-log-file)
  - [Custom Log Format](#custom-log-format)
  - [Controlling Log output coloring](#controlling-log-output-coloring)
  - [Model binding and validation](#model-binding-and-validation)
  - [Custom Validators](#custom-validators)
  - [Only Bind Query String](#only-bind-query-string)
  - [Bind Query String or Post Data](#bind-query-string-or-post-data)
  - [Bind default value if none provided](#bind-default-value-if-none-provided)
  - [Collection format for arrays](#collection-format-for-arrays)
  - [Bind Uri](#bind-uri)
  - [Bind custom unmarshaler](#bind-custom-unmarshaler)
  - [Bind Header](#bind-header)
  - [Bind HTML checkboxes](#bind-html-checkboxes)
  - [Multipart/Urlencoded binding](#multiparturlencoded-binding)
  - [XML, JSON, YAML, TOML and ProtoBuf rendering](#xml-json-yaml-toml-and-protobuf-rendering)
    - [SecureJSON](#securejson)
    - [JSONP](#jsonp)
    - [AsciiJSON](#asciijson)
    - [PureJSON](#purejson)
  - [Serving static files](#serving-static-files)
  - [Serving data from file](#serving-data-from-file)
  - [Serving data from reader](#serving-data-from-reader)
  - [HTML rendering](#html-rendering)
    - [Custom Template renderer](#custom-template-renderer)
    - [Custom Delimiters](#custom-delimiters)
    - [Custom Template Funcs](#custom-template-funcs)
  - [Multitemplate](#multitemplate)
  - [Redirects](#redirects)
  - [Custom Middleware](#custom-middleware)
  - [Using BasicAuth() middleware](#using-basicauth-middleware)
  - [Goroutines inside a middleware](#goroutines-inside-a-middleware)
  - [Custom HTTP configuration](#custom-http-configuration)
  - [Support Let's Encrypt](#support-lets-encrypt)
  - [Run multiple service using Gin](#run-multiple-service-using-gin)
  - [Graceful shutdown or restart](#graceful-shutdown-or-restart)
    - [Third-party packages](#third-party-packages)
    - [Manually](#manually)
  - [Build a single binary with templates](#build-a-single-binary-with-templates)
  - [Bind form-data request with custom struct](#bind-form-data-request-with-custom-struct)
  - [Try to bind body into different structs](#try-to-bind-body-into-different-structs)
  - [Bind form-data request with custom struct and custom tag](#bind-form-data-request-with-custom-struct-and-custom-tag)
  - [http2 server push](#http2-server-push)
  - [Define format for the log of routes](#define-format-for-the-log-of-routes)
  - [Set and get a cookie](#set-and-get-a-cookie)
  - [Custom json codec at runtime](#custom-json-codec-at-runtime)
- [Don't trust all proxies](#dont-trust-all-proxies)
- [Testing](#testing)

## Build tags

### Build with json replacement

Gin uses `encoding/json` as the default JSON package but you can change it by building from other tags.

[jsoniter](https://github.com/json-iterator/go)

```sh
go build -tags=jsoniter .
```

[go-json](https://github.com/goccy/go-json)

```sh
go build -tags=go_json .
```

[sonic](https://github.com/bytedance/sonic)

```sh
$ go build -tags=sonic .
```

### Build without `MsgPack` rendering feature

Gin enables `MsgPack` rendering feature by default. But you can disable this feature by specifying `nomsgpack` build tag.

```sh
go build -tags=nomsgpack .
```

This is useful to reduce the binary size of executable files. See the [detail information](https://github.com/gin-gonic/gin/pull/1852).

## API Examples

You can find a number of ready-to-run examples at [Gin examples repository](https://github.com/gin-gonic/examples).

### Using GET, POST, PUT, PATCH, DELETE and OPTIONS

```go
func main() {
  // Creates a gin router with default middleware:
  // logger and recovery (crash-free) middleware
  router := gin.Default()

  router.GET("/someGet", getting)
  router.POST("/somePost", posting)
  router.PUT("/somePut", putting)
  router.DELETE("/someDelete", deleting)
  router.PATCH("/somePatch", patching)
  router.HEAD("/someHead", head)
  router.OPTIONS("/someOptions", options)

  // By default, it serves on :8080 unless a
  // PORT environment variable was defined.
  router.Run()
  // router.Run(":3000") for a hard coded port
}
```

### Parameters in path

```go
func main() {
  router := gin.Default()

  // This handler will match /user/john but will not match /user/ or /user
  router.GET("/user/:name", func(c *gin.Context) {
    name := c.Param("name")
    c.String(http.StatusOK, "Hello %s", name)
  })

  // However, this one will match /user/john/ and also /user/john/send
  // If no other routers match /user/john, it will redirect to /user/john/
  router.GET("/user/:name/*action", func(c *gin.Context) {
    name := c.Param("name")
    action := c.Param("action")
    message := name + " is " + action
    c.String(http.StatusOK, message)
  })

  // For each matched request Context will hold the route definition
  router.POST("/user/:name/*action", func(c *gin.Context) {
    b := c.FullPath() == "/user/:name/*action" // true
    c.String(http.StatusOK, "%t", b)
  })

  // This handler will add a new router for /user/groups.
  // Exact routes are resolved before param routes, regardless of the order they were defined.
  // Routes starting with /user/groups are never interpreted as /user/:name/... routes
  router.GET("/user/groups", func(c *gin.Context) {
    c.String(http.StatusOK, "The available groups are [...]")
  })

  router.Run(":8080")
}
```

### Querystring parameters

```go
func main() {
  router := gin.Default()

  // Query string parameters are parsed using the existing underlying request object.
  // The request responds to a URL matching: /welcome?firstname=Jane&lastname=Doe
  router.GET("/welcome", func(c *gin.Context) {
    firstname := c.DefaultQuery("firstname", "Guest")
    lastname := c.Query("lastname") // shortcut for c.Request.URL.Query().Get("lastname")

    c.String(http.StatusOK, "Hello %s %s", firstname, lastname)
  })
  router.Run(":8080")
}
```

### Multipart/Urlencoded Form

```go
func main() {
  router := gin.Default()

  router.POST("/form_post", func(c *gin.Context) {
    message := c.PostForm("message")
    nick := c.DefaultPostForm("nick", "anonymous")

    c.JSON(http.StatusOK, gin.H{
      "status":  "posted",
      "message": message,
      "nick":    nick,
    })
  })
  router.Run(":8080")
}
```

### Another example: query + post form

```sh
POST /post?id=1234&page=1 HTTP/1.1
Content-Type: application/x-www-form-urlencoded

name=manu&message=this_is_great
```

```go
func main() {
  router := gin.Default()

  router.POST("/post", func(c *gin.Context) {

    id := c.Query("id")
    page := c.DefaultQuery("page", "0")
    name := c.PostForm("name")
    message := c.PostForm("message")

    fmt.Printf("id: %s; page: %s; name: %s; message: %s", id, page, name, message)
  })
  router.Run(":8080")
}
```

```sh
id: 1234; page: 1; name: manu; message: this_is_great
```

### Map as querystring or postform parameters

```sh
POST /post?ids[a]=1234&ids[b]=hello HTTP/1.1
Content-Type: application/x-www-form-urlencoded

names[first]=thinkerou&names[second]=tianou
```

```go
func main() {
  router := gin.Default()

  router.POST("/post", func(c *gin.Context) {

    ids := c.QueryMap("ids")
    names := c.PostFormMap("names")

    fmt.Printf("ids: %v; names: %v", ids, names)
  })
  router.Run(":8080")
}
```

```sh
ids: map[b:hello a:1234]; names: map[second:tianou first:thinkerou]
```

### Upload files

#### Single file

References issue [#774](https://github.com/gin-gonic/gin/issues/774) and detail [example code](https://github.com/gin-gonic/examples/tree/master/upload-file/single).

`file.Filename` **SHOULD NOT** be trusted. See [`Content-Disposition` on MDN](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition#Directives) and [#1693](https://github.com/gin-gonic/gin/issues/1693)

> The filename is always optional and must not be used blindly by the application: path information should be stripped, and conversion to the server file system rules should be done.

```go
func main() {
  router := gin.Default()
  // Set a lower memory limit for multipart forms (default is 32 MiB)
  router.MaxMultipartMemory = 8 << 20  // 8 MiB
  router.POST("/upload", func(c *gin.Context) {
    // Single file
    file, _ := c.FormFile("file")
    log.Println(file.Filename)

    // Upload the file to specific dst.
    c.SaveUploadedFile(file, dst)

    c.String(http.StatusOK, fmt.Sprintf("'%s' uploaded!", file.Filename))
  })
  router.Run(":8080")
}
```

How to `curl`:

```bash
curl -X POST http://localhost:8080/upload \
  -F "file=@/Users/appleboy/test.zip" \
  -H "Content-Type: multipart/form-data"
```

#### Multiple files

See the detailed [example code](https://github.com/gin-gonic/examples/tree/master/upload-file/multiple).

```go
func main() {
  router := gin.Default()
  // Set a lower memory limit for multipart forms (default is 32 MiB)
  router.MaxMultipartMemory = 8 << 20  // 8 MiB
  router.POST("/upload", func(c *gin.Context) {
    // Multipart form
    form, _ := c.MultipartForm()
    files := form.File["upload[]"]

    for _, file := range files {
      log.Println(file.Filename)

      // Upload the file to specific dst.
      c.SaveUploadedFile(file, dst)
    }
    c.String(http.StatusOK, fmt.Sprintf("%d files uploaded!", len(files)))
  })
  router.Run(":8080")
}
```

How to `curl`:

```bash
curl -X POST http://localhost:8080/upload \
  -F "upload[]=@/Users/appleboy/test1.zip" \
  -F "upload[]=@/Users/appleboy/test2.zip" \
  -H "Content-Type: multipart/form-data"
```

### Grouping routes

```go
func main() {
  router := gin.Default()

  // Simple group: v1
  {
    v1 := router.Group("/v1")
    v1.POST("/login", loginEndpoint)
    v1.POST("/submit", submitEndpoint)
    v1.POST("/read", readEndpoint)
  }

  // Simple group: v2
  {
    v2 := router.Group("/v2")
    v2.POST("/login", loginEndpoint)
    v2.POST("/submit", submitEndpoint)
    v2.POST("/read", readEndpoint)
  }

  router.Run(":8080")
}
```

### Blank Gin without middleware by default

Use

```go
r := gin.New()
```

instead of

```go
// Default With the Logger and Recovery middleware already attached
r := gin.Default()
```

### Using middleware

```go
func main() {
  // Creates a router without any middleware by default
  r := gin.New()

  // Global middleware
  // Logger middleware will write the logs to gin.DefaultWriter even if you set with GIN_MODE=release.
  // By default gin.DefaultWriter = os.Stdout
  r.Use(gin.Logger())

  // Recovery middleware recovers from any panics and writes a 500 if there was one.
  r.Use(gin.Recovery())

  // Per route middleware, you can add as many as you desire.
  r.GET("/benchmark", MyBenchLogger(), benchEndpoint)

  // Authorization group
  // authorized := r.Group("/", AuthRequired())
  // exactly the same as:
  authorized := r.Group("/")
  // per group middleware! in this case we use the custom created
  // AuthRequired() middleware just in the "authorized" group.
  authorized.Use(AuthRequired())
  {
    authorized.POST("/login", loginEndpoint)
    authorized.POST("/submit", submitEndpoint)
    authorized.POST("/read", readEndpoint)

    // nested group
    testing := authorized.Group("testing")
    // visit 0.0.0.0:8080/testing/analytics
    testing.GET("/analytics", analyticsEndpoint)
  }

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

### Custom Recovery behavior

```go
func main() {
  // Creates a router without any middleware by default
  r := gin.New()

  // Global middleware
  // Logger middleware will write the logs to gin.DefaultWriter even if you set with GIN_MODE=release.
  // By default gin.DefaultWriter = os.Stdout
  r.Use(gin.Logger())

  // Recovery middleware recovers from any panics and writes a 500 if there was one.
  r.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
    if err, ok := recovered.(string); ok {
      c.String(http.StatusInternalServerError, fmt.Sprintf("error: %s", err))
    }
    c.AbortWithStatus(http.StatusInternalServerError)
  }))

  r.GET("/panic", func(c *gin.Context) {
    // panic with a string -- the custom middleware could save this to a database or report it to the user
    panic("foo")
  })

  r.GET("/", func(c *gin.Context) {
    c.String(http.StatusOK, "ohai")
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

### How to write log file

```go
func main() {
  // Disable Console Color, you don't need console color when writing the logs to file.
  gin.DisableConsoleColor()

  // Logging to a file.
  f, _ := os.Create("gin.log")
  gin.DefaultWriter = io.MultiWriter(f)

  // Use the following code if you need to write the logs to file and console at the same time.
  // gin.DefaultWriter = io.MultiWriter(f, os.Stdout)

  router := gin.Default()
  router.GET("/ping", func(c *gin.Context) {
      c.String(http.StatusOK, "pong")
  })

   router.Run(":8080")
}
```

### Custom Log Format

```go
func main() {
  router := gin.New()

  // LoggerWithFormatter middleware will write the logs to gin.DefaultWriter
  // By default gin.DefaultWriter = os.Stdout
  router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {

    // your custom format
    return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
        param.ClientIP,
        param.TimeStamp.Format(time.RFC1123),
        param.Method,
        param.Path,
        param.Request.Proto,
        param.StatusCode,
        param.Latency,
        param.Request.UserAgent(),
        param.ErrorMessage,
    )
  }))
  router.Use(gin.Recovery())

  router.GET("/ping", func(c *gin.Context) {
    c.String(http.StatusOK, "pong")
  })

  router.Run(":8080")
}
```

Sample Output

```sh
::1 - [Fri, 07 Dec 2018 17:04:38 JST] "GET /ping HTTP/1.1 200 122.767µs "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/71.0.3578.80 Safari/537.36" "
```

### Skip logging

```go
func main() {
  router := gin.New()

  // skip logging for desired paths by setting SkipPaths in LoggerConfig
  loggerConfig := gin.LoggerConfig{SkipPaths: []string{"/metrics"}}

  // skip logging based on your logic by setting Skip func in LoggerConfig
  loggerConfig.Skip = func(c *gin.Context) bool {
      // as an example skip non server side errors
      return c.Writer.Status() < http.StatusInternalServerError
  }

  router.Use(gin.LoggerWithConfig(loggerConfig))
  router.Use(gin.Recovery())

  // skipped
  router.GET("/metrics", func(c *gin.Context) {
      c.Status(http.StatusNotImplemented)
  })

  // skipped
  router.GET("/ping", func(c *gin.Context) {
      c.String(http.StatusOK, "pong")
  })

  // not skipped
  router.GET("/data", func(c *gin.Context) {
    c.Status(http.StatusNotImplemented)
  })

  router.Run(":8080")
}

```

### Controlling Log output coloring

By default, logs output on console should be colorized depending on the detected TTY.

Never colorize logs:

```go
func main() {
  // Disable log's color
  gin.DisableConsoleColor()

  // Creates a gin router with default middleware:
  // logger and recovery (crash-free) middleware
  router := gin.Default()

  router.GET("/ping", func(c *gin.Context) {
      c.String(http.StatusOK, "pong")
  })

  router.Run(":8080")
}
```

Always colorize logs:

```go
func main() {
  // Force log's color
  gin.ForceConsoleColor()

  // Creates a gin router with default middleware:
  // logger and recovery (crash-free) middleware
  router := gin.Default()

  router.GET("/ping", func(c *gin.Context) {
      c.String(http.StatusOK, "pong")
  })

  router.Run(":8080")
}
```

### Model binding and validation

To bind a request body into a type, use model binding. We currently support binding of JSON, XML, YAML, TOML and standard form values (foo=bar&boo=baz).

Gin uses [**go-playground/validator/v10**](https://github.com/go-playground/validator) for validation. Check the full docs on tags usage [here](https://pkg.go.dev/github.com/go-playground/validator#hdr-Baked_In_Validators_and_Tags).

Note that you need to set the corresponding binding tag on all fields you want to bind. For example, when binding from JSON, set `json:"fieldname"`.

Also, Gin provides two sets of methods for binding:

- **Type** - Must bind
  - **Methods** - `Bind`, `BindJSON`, `BindXML`, `BindQuery`, `BindYAML`, `BindHeader`, `BindTOML`
  - **Behavior** - These methods use `MustBindWith` under the hood. If there is a binding error, the request is aborted with `c.AbortWithError(400, err).SetType(ErrorTypeBind)`. This sets the response status code to 400 and the `Content-Type` header is set to `text/plain; charset=utf-8`. Note that if you try to set the response code after this, it will result in a warning `[GIN-debug] [WARNING] Headers were already written. Wanted to override status code 400 with 422`. If you wish to have greater control over the behavior, consider using the `ShouldBind` equivalent method.
- **Type** - Should bind
  - **Methods** - `ShouldBind`, `ShouldBindJSON`, `ShouldBindXML`, `ShouldBindQuery`, `ShouldBindYAML`, `ShouldBindHeader`, `ShouldBindTOML`,
  - **Behavior** - These methods use `ShouldBindWith` under the hood. If there is a binding error, the error is returned and it is the developer's responsibility to handle the request and error appropriately.

When using the Bind-method, Gin tries to infer the binder depending on the Content-Type header. If you are sure what you are binding, you can use `MustBindWith` or `ShouldBindWith`.

You can also specify that specific fields are required. If a field is decorated with `binding:"required"` and has an empty value when binding, an error will be returned.

```go
// Binding from JSON
type Login struct {
  User     string `form:"user" json:"user" xml:"user" binding:"required"`
  Password string `form:"password" json:"password" xml:"password" binding:"required"`
}

func main() {
  router := gin.Default()

  // Example for binding JSON ({"user": "manu", "password": "123"})
  router.POST("/loginJSON", func(c *gin.Context) {
    var json Login
    if err := c.ShouldBindJSON(&json); err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      return
    }

    if json.User != "manu" || json.Password != "123" {
      c.JSON(http.StatusUnauthorized, gin.H{"status": "unauthorized"})
      return
    }

    c.JSON(http.StatusOK, gin.H{"status": "you are logged in"})
  })

  // Example for binding XML (
  //  <?xml version="1.0" encoding="UTF-8"?>
  //  <root>
  //    <user>manu</user>
  //    <password>123</password>
  //  </root>)
  router.POST("/loginXML", func(c *gin.Context) {
    var xml Login
    if err := c.ShouldBindXML(&xml); err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      return
    }

    if xml.User != "manu" || xml.Password != "123" {
      c.JSON(http.StatusUnauthorized, gin.H{"status": "unauthorized"})
      return
    }

    c.JSON(http.StatusOK, gin.H{"status": "you are logged in"})
  })

  // Example for binding a HTML form (user=manu&password=123)
  router.POST("/loginForm", func(c *gin.Context) {
    var form Login
    // This will infer what binder to use depending on the content-type header.
    if err := c.ShouldBind(&form); err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      return
    }

    if form.User != "manu" || form.Password != "123" {
      c.JSON(http.StatusUnauthorized, gin.H{"status": "unauthorized"})
      return
    }

    c.JSON(http.StatusOK, gin.H{"status": "you are logged in"})
  })

  // Listen and serve on 0.0.0.0:8080
  router.Run(":8080")
}
```

Sample request

```sh
$ curl -v -X POST \
  http://localhost:8080/loginJSON \
  -H 'content-type: application/json' \
  -d '{ "user": "manu" }'
> POST /loginJSON HTTP/1.1
> Host: localhost:8080
> User-Agent: curl/7.51.0
> Accept: */*
> content-type: application/json
> Content-Length: 18
>
* upload completely sent off: 18 out of 18 bytes
< HTTP/1.1 400 Bad Request
< Content-Type: application/json; charset=utf-8
< Date: Fri, 04 Aug 2017 03:51:31 GMT
< Content-Length: 100
<
{"error":"Key: 'Login.Password' Error:Field validation for 'Password' failed on the 'required' tag"}
```

Skip-validation: Running the example above using the `curl` command returns an error. This is because the example uses `binding:"required"` for `Password`. If instead, you use `binding:"-"` for `Password`, then it will not return an error when you run the example again.

### Custom Validators

It is also possible to register custom validators. See the [example code](https://github.com/gin-gonic/examples/tree/master/custom-validation/server.go).

```go
package main

import (
  "net/http"
  "time"

  "github.com/gin-gonic/gin"
  "github.com/gin-gonic/gin/binding"
  "github.com/go-playground/validator/v10"
)

// Booking contains binded and validated data.
type Booking struct {
  CheckIn  time.Time `form:"check_in" binding:"required,bookabledate" time_format:"2006-01-02"`
  CheckOut time.Time `form:"check_out" binding:"required,gtfield=CheckIn" time_format:"2006-01-02"`
}

var bookableDate validator.Func = func(fl validator.FieldLevel) bool {
  date, ok := fl.Field().Interface().(time.Time)
  if ok {
    today := time.Now()
    if today.After(date) {
      return false
    }
  }
  return true
}

func main() {
  route := gin.Default()

  if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
    v.RegisterValidation("bookabledate", bookableDate)
  }

  route.GET("/bookable", getBookable)
  route.Run(":8085")
}

func getBookable(c *gin.Context) {
  var b Booking
  if err := c.ShouldBindWith(&b, binding.Query); err == nil {
    c.JSON(http.StatusOK, gin.H{"message": "Booking dates are valid!"})
  } else {
    c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
  }
}
```

```console
$ curl "localhost:8085/bookable?check_in=2030-04-16&check_out=2030-04-17"
{"message":"Booking dates are valid!"}

$ curl "localhost:8085/bookable?check_in=2030-03-10&check_out=2030-03-09"
{"error":"Key: 'Booking.CheckOut' Error:Field validation for 'CheckOut' failed on the 'gtfield' tag"}

$ curl "localhost:8085/bookable?check_in=2000-03-09&check_out=2000-03-10"
{"error":"Key: 'Booking.CheckIn' Error:Field validation for 'CheckIn' failed on the 'bookabledate' tag"}%
```

[Struct level validations](https://github.com/go-playground/validator/releases/tag/v8.7) can also be registered this way.
See the [struct-lvl-validation example](https://github.com/gin-gonic/examples/tree/master/struct-lvl-validations) to learn more.

### Only Bind Query String

`ShouldBindQuery` function only binds the query params and not the post data. See the [detail information](https://github.com/gin-gonic/gin/issues/742#issuecomment-315953017).

```go
package main

import (
  "log"
  "net/http"

  "github.com/gin-gonic/gin"
)

type Person struct {
  Name    string `form:"name"`
  Address string `form:"address"`
}

func main() {
  route := gin.Default()
  route.Any("/testing", startPage)
  route.Run(":8085")
}

func startPage(c *gin.Context) {
  var person Person
  if c.ShouldBindQuery(&person) == nil {
    log.Println("====== Only Bind By Query String ======")
    log.Println(person.Name)
    log.Println(person.Address)
  }
  c.String(http.StatusOK, "Success")
}

```

### Bind Query String or Post Data

See the [detail information](https://github.com/gin-gonic/gin/issues/742#issuecomment-264681292).

```go
package main

import (
  "log"
  "net/http"
  "time"

  "github.com/gin-gonic/gin"
)

type Person struct {
  Name       string    `form:"name"`
  Address    string    `form:"address"`
  Birthday   time.Time `form:"birthday" time_format:"2006-01-02" time_utc:"1"`
  CreateTime time.Time `form:"createTime" time_format:"unixNano"`
  UnixTime   time.Time `form:"unixTime" time_format:"unix"`
  UnixMilliTime   time.Time `form:"unixMilliTime" time_format:"unixmilli"`
  UnixMicroTime   time.Time `form:"unixMicroTime" time_format:"uNiXmIcRo"` // case does not matter for "unix*" time formats
}

func main() {
  route := gin.Default()
  route.GET("/testing", startPage)
  route.Run(":8085")
}

func startPage(c *gin.Context) {
  var person Person
  // If `GET`, only `Form` binding engine (`query`) used.
  // If `POST`, first checks the `content-type` for `JSON` or `XML`, then uses `Form` (`form-data`).
  // See more at https://github.com/gin-gonic/gin/blob/master/binding/binding.go#L88
  if c.ShouldBind(&person) == nil {
    log.Println(person.Name)
    log.Println(person.Address)
    log.Println(person.Birthday)
    log.Println(person.CreateTime)
    log.Println(person.UnixTime)
    log.Println(person.UnixMilliTime)
    log.Println(person.UnixMicroTime)
  }

  c.String(http.StatusOK, "Success")
}
```

Test it with:

```sh
curl -X GET "localhost:8085/testing?name=appleboy&address=xyz&birthday=1992-03-15&createTime=1562400033000000123&unixTime=1562400033&unixMilliTime=1562400033001&unixMicroTime=1562400033000012"
```


### Bind default value if none provided

If the server should bind a default value to a field when the client does not provide one, specify the default value using the `default` key within the `form` tag:

```go
package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Person struct {
	Name      string    `form:"name,default=William"`
	Age       int       `form:"age,default=10"`
	Friends   []string  `form:"friends,default=Will;Bill"`
	Addresses [2]string `form:"addresses,default=foo bar" collection_format:"ssv"`
	LapTimes  []int     `form:"lap_times,default=1;2;3" collection_format:"csv"`
}

func main() {
	g := gin.Default()
	g.POST("/person", func(c *gin.Context) {
		var req Person
		if err := c.ShouldBindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, err)
			return
		}
		c.JSON(http.StatusOK, req)
	})
	_ = g.Run("localhost:8080")
}
```

```
curl -X POST http://localhost:8080/person
{"Name":"William","Age":10,"Friends":["Will","Bill"],"Colors":["red","blue"],"LapTimes":[1,2,3]}
```

NOTE: For default [collection values](#collection-format-for-arrays), the following rules apply:
- Since commas are used to delimit tag options, they are not supported within a default value and will result in undefined behavior
- For the collection formats "multi" and "csv", a semicolon should be used in place of a comma to delimited default values
- Since semicolons are used to delimit default values for "multi" and "csv", they are not supported within a default value for "multi" and "csv"


#### Collection format for arrays

| Format          | Description                                               | Example                 |
| --------------- | --------------------------------------------------------- | ----------------------- |
| multi (default) | Multiple parameter instances rather than multiple values. | key=foo&key=bar&key=baz |
| csv             | Comma-separated values.                                   | foo,bar,baz             |
| ssv             | Space-separated values.                                   | foo bar baz             |
| tsv             | Tab-separated values.                                     | "foo\tbar\tbaz"         |
| pipes           | Pipe-separated values.                                    | foo\|bar\|baz           |

```go
package main

import (
	"log"
	"time"
	"github.com/gin-gonic/gin"
)

type Person struct {
	Name       string    `form:"name"`
	Addresses  []string  `form:"addresses" collection_format:"csv"`
	Birthday   time.Time `form:"birthday" time_format:"2006-01-02" time_utc:"1"`
	CreateTime time.Time `form:"createTime" time_format:"unixNano"`
	UnixTime   time.Time `form:"unixTime" time_format:"unix"`
}

func main() {
	route := gin.Default()
	route.GET("/testing", startPage)
	route.Run(":8085")
}
func startPage(c *gin.Context) {
	var person Person
	// If `GET`, only `Form` binding engine (`query`) used.
	// If `POST`, first checks the `content-type` for `JSON` or `XML`, then uses `Form` (`form-data`).
	// See more at https://github.com/gin-gonic/gin/blob/master/binding/binding.go#L48
        if c.ShouldBind(&person) == nil {
                log.Println(person.Name)
                log.Println(person.Addresses)
                log.Println(person.Birthday)
                log.Println(person.CreateTime)
                log.Println(person.UnixTime)
        }
	c.String(200, "Success")
}
```

Test it with:
```sh
$ curl -X GET "localhost:8085/testing?name=appleboy&addresses=foo,bar&birthday=1992-03-15&createTime=1562400033000000123&unixTime=1562400033"
```

### Bind Uri

See the [detail information](https://github.com/gin-gonic/gin/issues/846).

```go
package main

import (
  "net/http"

  "github.com/gin-gonic/gin"
)

type Person struct {
  ID string `uri:"id" binding:"required,uuid"`
  Name string `uri:"name" binding:"required"`
}

func main() {
  route := gin.Default()
  route.GET("/:name/:id", func(c *gin.Context) {
    var person Person
    if err := c.ShouldBindUri(&person); err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"msg": err.Error()})
      return
    }
    c.JSON(http.StatusOK, gin.H{"name": person.Name, "uuid": person.ID})
  })
  route.Run(":8088")
}
```

Test it with:

```sh
curl -v localhost:8088/thinkerou/987fbc97-4bed-5078-9f07-9141ba07c9f3
curl -v localhost:8088/thinkerou/not-uuid
```

### Bind custom unmarshaler

```go
package main

import (
  "github.com/gin-gonic/gin"
  "strings"
)

type Birthday string

func (b *Birthday) UnmarshalParam(param string) error {
  *b = Birthday(strings.Replace(param, "-", "/", -1))
  return nil
}

func main() {
  route := gin.Default()
  var request struct {
    Birthday Birthday `form:"birthday"`
  }
  route.GET("/test", func(ctx *gin.Context) {
    _ = ctx.BindQuery(&request)
    ctx.JSON(200, request.Birthday)
  })
  route.Run(":8088")
}
```

Test it with:

```sh
curl 'localhost:8088/test?birthday=2000-01-01'
```
Result
```sh
"2000/01/01"
```

### Bind Header

```go
package main

import (
  "fmt"
  "net/http"

  "github.com/gin-gonic/gin"
)

type testHeader struct {
  Rate   int    `header:"Rate"`
  Domain string `header:"Domain"`
}

func main() {
  r := gin.Default()
  r.GET("/", func(c *gin.Context) {
    h := testHeader{}

    if err := c.ShouldBindHeader(&h); err != nil {
      c.JSON(http.StatusOK, err)
    }

    fmt.Printf("%#v\n", h)
    c.JSON(http.StatusOK, gin.H{"Rate": h.Rate, "Domain": h.Domain})
  })

  r.Run()

// client
// curl -H "rate:300" -H "domain:music" 127.0.0.1:8080/
// output
// {"Domain":"music","Rate":300}
}
```

### Bind HTML checkboxes

See the [detail information](https://github.com/gin-gonic/gin/issues/129#issuecomment-124260092)

main.go

```go
...

type myForm struct {
    Colors []string `form:"colors[]"`
}

...

func formHandler(c *gin.Context) {
    var fakeForm myForm
    c.ShouldBind(&fakeForm)
    c.JSON(http.StatusOK, gin.H{"color": fakeForm.Colors})
}

...

```

form.html

```html
<form action="/" method="POST">
    <p>Check some colors</p>
    <label for="red">Red</label>
    <input type="checkbox" name="colors[]" value="red" id="red">
    <label for="green">Green</label>
    <input type="checkbox" name="colors[]" value="green" id="green">
    <label for="blue">Blue</label>
    <input type="checkbox" name="colors[]" value="blue" id="blue">
    <input type="submit">
</form>
```

result:

```json
{"color":["red","green","blue"]}
```

### Multipart/Urlencoded binding

```go
type ProfileForm struct {
  Name   string                `form:"name" binding:"required"`
  Avatar *multipart.FileHeader `form:"avatar" binding:"required"`

  // or for multiple files
  // Avatars []*multipart.FileHeader `form:"avatar" binding:"required"`
}

func main() {
  router := gin.Default()
  router.POST("/profile", func(c *gin.Context) {
    // you can bind multipart form with explicit binding declaration:
    // c.ShouldBindWith(&form, binding.Form)
    // or you can simply use autobinding with ShouldBind method:
    var form ProfileForm
    // in this case proper binding will be automatically selected
    if err := c.ShouldBind(&form); err != nil {
      c.String(http.StatusBadRequest, "bad request")
      return
    }

    err := c.SaveUploadedFile(form.Avatar, form.Avatar.Filename)
    if err != nil {
      c.String(http.StatusInternalServerError, "unknown error")
      return
    }

    // db.Save(&form)

    c.String(http.StatusOK, "ok")
  })
  router.Run(":8080")
}
```

Test it with:

```sh
curl -X POST -v --form name=user --form "avatar=@./avatar.png" http://localhost:8080/profile
```

### XML, JSON, YAML, TOML and ProtoBuf rendering

```go
func main() {
  r := gin.Default()

  // gin.H is a shortcut for map[string]any
  r.GET("/someJSON", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"message": "hey", "status": http.StatusOK})
  })

  r.GET("/moreJSON", func(c *gin.Context) {
    // You can also use a struct
    var msg struct {
      Name    string `json:"user"`
      Message string
      Number  int
    }
    msg.Name = "Lena"
    msg.Message = "hey"
    msg.Number = 123
    // Note that msg.Name becomes "user" in the JSON
    // Will output  :   {"user": "Lena", "Message": "hey", "Number": 123}
    c.JSON(http.StatusOK, msg)
  })

  r.GET("/someXML", func(c *gin.Context) {
    c.XML(http.StatusOK, gin.H{"message": "hey", "status": http.StatusOK})
  })

  r.GET("/someYAML", func(c *gin.Context) {
    c.YAML(http.StatusOK, gin.H{"message": "hey", "status": http.StatusOK})
  })

  r.GET("/someTOML", func(c *gin.Context) {
    c.TOML(http.StatusOK, gin.H{"message": "hey", "status": http.StatusOK})
  })

  r.GET("/someProtoBuf", func(c *gin.Context) {
    reps := []int64{int64(1), int64(2)}
    label := "test"
    // The specific definition of protobuf is written in the testdata/protoexample file.
    data := &protoexample.Test{
      Label: &label,
      Reps:  reps,
    }
    // Note that data becomes binary data in the response
    // Will output protoexample.Test protobuf serialized data
    c.ProtoBuf(http.StatusOK, data)
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

#### SecureJSON

Using SecureJSON to prevent json hijacking. Default prepends `"while(1),"` to response body if the given struct is array values.

```go
func main() {
  r := gin.Default()

  // You can also use your own secure json prefix
  // r.SecureJsonPrefix(")]}',\n")

  r.GET("/someJSON", func(c *gin.Context) {
    names := []string{"lena", "austin", "foo"}

    // Will output  :   while(1);["lena","austin","foo"]
    c.SecureJSON(http.StatusOK, names)
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

#### JSONP

Using JSONP to request data from a server in a different domain. Add callback to response body if the query parameter callback exists.

```go
func main() {
  r := gin.Default()

  r.GET("/JSONP", func(c *gin.Context) {
    data := gin.H{
      "foo": "bar",
    }

    //callback is x
    // Will output  :   x({\"foo\":\"bar\"})
    c.JSONP(http.StatusOK, data)
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")

        // client
        // curl http://127.0.0.1:8080/JSONP?callback=x
}
```

#### AsciiJSON

Using AsciiJSON to Generates ASCII-only JSON with escaped non-ASCII characters.

```go
func main() {
  r := gin.Default()

  r.GET("/someJSON", func(c *gin.Context) {
    data := gin.H{
      "lang": "GO语言",
      "tag":  "<br>",
    }

    // will output : {"lang":"GO\u8bed\u8a00","tag":"\u003cbr\u003e"}
    c.AsciiJSON(http.StatusOK, data)
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

#### PureJSON

Normally, JSON replaces special HTML characters with their unicode entities, e.g. `<` becomes `\u003c`. If you want to encode such characters literally, you can use PureJSON instead.
This feature is unavailable in Go 1.6 and lower.

```go
func main() {
  r := gin.Default()

  // Serves unicode entities
  r.GET("/json", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
      "html": "<b>Hello, world!</b>",
    })
  })

  // Serves literal characters
  r.GET("/purejson", func(c *gin.Context) {
    c.PureJSON(http.StatusOK, gin.H{
      "html": "<b>Hello, world!</b>",
    })
  })

  // listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

### Serving static files

```go
func main() {
  router := gin.Default()
  router.Static("/assets", "./assets")
  router.StaticFS("/more_static", http.Dir("my_file_system"))
  router.StaticFile("/favicon.ico", "./resources/favicon.ico")
  router.StaticFileFS("/more_favicon.ico", "more_favicon.ico", http.Dir("my_file_system"))

  // Listen and serve on 0.0.0.0:8080
  router.Run(":8080")
}
```

### Serving data from file

```go
func main() {
  router := gin.Default()

  router.GET("/local/file", func(c *gin.Context) {
    c.File("local/file.go")
  })

  var fs http.FileSystem = // ...
  router.GET("/fs/file", func(c *gin.Context) {
    c.FileFromFS("fs/file.go", fs)
  })
}

```

### Serving data from reader

```go
func main() {
  router := gin.Default()
  router.GET("/someDataFromReader", func(c *gin.Context) {
    response, err := http.Get("https://raw.githubusercontent.com/gin-gonic/logo/master/color.png")
    if err != nil || response.StatusCode != http.StatusOK {
      c.Status(http.StatusServiceUnavailable)
      return
    }

    reader := response.Body
     defer reader.Close()
    contentLength := response.ContentLength
    contentType := response.Header.Get("Content-Type")

    extraHeaders := map[string]string{
      "Content-Disposition": `attachment; filename="gopher.png"`,
    }

    c.DataFromReader(http.StatusOK, contentLength, contentType, reader, extraHeaders)
  })
  router.Run(":8080")
}
```

### HTML rendering

Using LoadHTMLGlob() or LoadHTMLFiles() or LoadHTMLFS()

```go
//go:embed templates/*
var templates embed.FS

func main() {
  router := gin.Default()
  router.LoadHTMLGlob("templates/*")
  //router.LoadHTMLFiles("templates/template1.html", "templates/template2.html")
  //router.LoadHTMLFS(http.Dir("templates"), "template1.html", "template2.html")
  //or
  //router.LoadHTMLFS(http.FS(templates), "templates/template1.html", "templates/template2.html")
  router.GET("/index", func(c *gin.Context) {
    c.HTML(http.StatusOK, "index.tmpl", gin.H{
      "title": "Main website",
    })
  })
  router.Run(":8080")
}
```

templates/index.tmpl

```html
<html>
  <h1>
    {{ .title }}
  </h1>
</html>
```

Using templates with same name in different directories

```go
func main() {
  router := gin.Default()
  router.LoadHTMLGlob("templates/**/*")
  router.GET("/posts/index", func(c *gin.Context) {
    c.HTML(http.StatusOK, "posts/index.tmpl", gin.H{
      "title": "Posts",
    })
  })
  router.GET("/users/index", func(c *gin.Context) {
    c.HTML(http.StatusOK, "users/index.tmpl", gin.H{
      "title": "Users",
    })
  })
  router.Run(":8080")
}
```

templates/posts/index.tmpl

```html
{{ define "posts/index.tmpl" }}
<html><h1>
  {{ .title }}
</h1>
<p>Using posts/index.tmpl</p>
</html>
{{ end }}
```

templates/users/index.tmpl

```html
{{ define "users/index.tmpl" }}
<html><h1>
  {{ .title }}
</h1>
<p>Using users/index.tmpl</p>
</html>
{{ end }}
```

#### Custom Template renderer

You can also use your own html template render

```go
import "html/template"

func main() {
  router := gin.Default()
  html := template.Must(template.ParseFiles("file1", "file2"))
  router.SetHTMLTemplate(html)
  router.Run(":8080")
}
```

#### Custom Delimiters

You may use custom delims

```go
  r := gin.Default()
  r.Delims("{[{", "}]}")
  r.LoadHTMLGlob("/path/to/templates")
```

#### Custom Template Funcs

See the detailed [example code](https://github.com/gin-gonic/examples/tree/master/template).

main.go

```go
import (
  "fmt"
  "html/template"
  "net/http"
  "time"

  "github.com/gin-gonic/gin"
)

func formatAsDate(t time.Time) string {
  year, month, day := t.Date()
  return fmt.Sprintf("%d/%02d/%02d", year, month, day)
}

func main() {
  router := gin.Default()
  router.Delims("{[{", "}]}")
  router.SetFuncMap(template.FuncMap{
      "formatAsDate": formatAsDate,
  })
  router.LoadHTMLFiles("./testdata/template/raw.tmpl")

  router.GET("/raw", func(c *gin.Context) {
      c.HTML(http.StatusOK, "raw.tmpl", gin.H{
          "now": time.Date(2017, 07, 01, 0, 0, 0, 0, time.UTC),
      })
  })

  router.Run(":8080")
}

```

raw.tmpl

```html
Date: {[{.now | formatAsDate}]}
```

Result:

```sh
Date: 2017/07/01
```

### Multitemplate

Gin allows only one html.Template by default. Check [a multitemplate render](https://github.com/gin-contrib/multitemplate) for using features like go 1.6 `block template`.

### Redirects

Issuing a HTTP redirect is easy. Both internal and external locations are supported.

```go
r.GET("/test", func(c *gin.Context) {
  c.Redirect(http.StatusMovedPermanently, "http://www.google.com/")
})
```

Issuing a HTTP redirect from POST. Refer to issue: [#444](https://github.com/gin-gonic/gin/issues/444)

```go
r.POST("/test", func(c *gin.Context) {
  c.Redirect(http.StatusFound, "/foo")
})
```

Issuing a Router redirect, use `HandleContext` like below.

``` go
r.GET("/test", func(c *gin.Context) {
    c.Request.URL.Path = "/test2"
    r.HandleContext(c)
})
r.GET("/test2", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"hello": "world"})
})
```

### Custom Middleware

```go
func Logger() gin.HandlerFunc {
  return func(c *gin.Context) {
    t := time.Now()

    // Set example variable
    c.Set("example", "12345")

    // before request

    c.Next()

    // after request
    latency := time.Since(t)
    log.Print(latency)

    // access the status we are sending
    status := c.Writer.Status()
    log.Println(status)
  }
}

func main() {
  r := gin.New()
  r.Use(Logger())

  r.GET("/test", func(c *gin.Context) {
    example := c.MustGet("example").(string)

    // it would print: "12345"
    log.Println(example)
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

### Using BasicAuth() middleware

```go
// simulate some private data
var secrets = gin.H{
  "foo":    gin.H{"email": "foo@bar.com", "phone": "123433"},
  "austin": gin.H{"email": "austin@example.com", "phone": "666"},
  "lena":   gin.H{"email": "lena@guapa.com", "phone": "523443"},
}

func main() {
  r := gin.Default()

  // Group using gin.BasicAuth() middleware
  // gin.Accounts is a shortcut for map[string]string
  authorized := r.Group("/admin", gin.BasicAuth(gin.Accounts{
    "foo":    "bar",
    "austin": "1234",
    "lena":   "hello2",
    "manu":   "4321",
  }))

  // /admin/secrets endpoint
  // hit "localhost:8080/admin/secrets
  authorized.GET("/secrets", func(c *gin.Context) {
    // get user, it was set by the BasicAuth middleware
    user := c.MustGet(gin.AuthUserKey).(string)
    if secret, ok := secrets[user]; ok {
      c.JSON(http.StatusOK, gin.H{"user": user, "secret": secret})
    } else {
      c.JSON(http.StatusOK, gin.H{"user": user, "secret": "NO SECRET :("})
    }
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

### Goroutines inside a middleware

When starting new Goroutines inside a middleware or handler, you **SHOULD NOT** use the original context inside it, you have to use a read-only copy.

```go
func main() {
  r := gin.Default()

  r.GET("/long_async", func(c *gin.Context) {
    // create copy to be used inside the goroutine
    cCp := c.Copy()
    go func() {
      // simulate a long task with time.Sleep(). 5 seconds
      time.Sleep(5 * time.Second)

      // note that you are using the copied context "cCp", IMPORTANT
      log.Println("Done! in path " + cCp.Request.URL.Path)
    }()
  })

  r.GET("/long_sync", func(c *gin.Context) {
    // simulate a long task with time.Sleep(). 5 seconds
    time.Sleep(5 * time.Second)

    // since we are NOT using a goroutine, we do not have to copy the context
    log.Println("Done! in path " + c.Request.URL.Path)
  })

  // Listen and serve on 0.0.0.0:8080
  r.Run(":8080")
}
```

### Custom HTTP configuration

Use `http.ListenAndServe()` directly, like this:

```go
func main() {
  router := gin.Default()
  http.ListenAndServe(":8080", router)
}
```

or

```go
func main() {
  router := gin.Default()

  s := &http.Server{
    Addr:           ":8080",
    Handler:        router,
    ReadTimeout:    10 * time.Second,
    WriteTimeout:   10 * time.Second,
    MaxHeaderBytes: 1 << 20,
  }
  s.ListenAndServe()
}
```

### Support Let's Encrypt

example for 1-line LetsEncrypt HTTPS servers.

```go
package main

import (
  "log"
  "net/http"

  "github.com/gin-gonic/autotls"
  "github.com/gin-gonic/gin"
)

func main() {
  r := gin.Default()

  // Ping handler
  r.GET("/ping", func(c *gin.Context) {
    c.String(http.StatusOK, "pong")
  })

  log.Fatal(autotls.Run(r, "example1.com", "example2.com"))
}
```

example for custom autocert manager.

```go
package main

import (
  "log"
  "net/http"

  "github.com/gin-gonic/autotls"
  "github.com/gin-gonic/gin"
  "golang.org/x/crypto/acme/autocert"
)

func main() {
  r := gin.Default()

  // Ping handler
  r.GET("/ping", func(c *gin.Context) {
    c.String(http.StatusOK, "pong")
  })

  m := autocert.Manager{
    Prompt:     autocert.AcceptTOS,
    HostPolicy: autocert.HostWhitelist("example1.com", "example2.com"),
    Cache:      autocert.DirCache("/var/www/.cache"),
  }

  log.Fatal(autotls.RunWithManager(r, &m))
}
```

### Run multiple service using Gin

See the [question](https://github.com/gin-gonic/gin/issues/346) and try the following example:

```go
package main

import (
  "log"
  "net/http"
  "time"

  "github.com/gin-gonic/gin"
  "golang.org/x/sync/errgroup"
)

var (
  g errgroup.Group
)

func router01() http.Handler {
  e := gin.New()
  e.Use(gin.Recovery())
  e.GET("/", func(c *gin.Context) {
    c.JSON(
      http.StatusOK,
      gin.H{
        "code":  http.StatusOK,
        "error": "Welcome server 01",
      },
    )
  })

  return e
}

func router02() http.Handler {
  e := gin.New()
  e.Use(gin.Recovery())
  e.GET("/", func(c *gin.Context) {
    c.JSON(
      http.StatusOK,
      gin.H{
        "code":  http.StatusOK,
        "error": "Welcome server 02",
      },
    )
  })

  return e
}

func main() {
  server01 := &http.Server{
    Addr:         ":8080",
    Handler:      router01(),
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
  }

  server02 := &http.Server{
    Addr:         ":8081",
    Handler:      router02(),
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 10 * time.Second,
  }

  g.Go(func() error {
    err := server01.ListenAndServe()
    if err != nil && err != http.ErrServerClosed {
      log.Fatal(err)
    }
    return err
  })

  g.Go(func() error {
    err := server02.ListenAndServe()
    if err != nil && err != http.ErrServerClosed {
      log.Fatal(err)
    }
    return err
  })

  if err := g.Wait(); err != nil {
    log.Fatal(err)
  }
}
```

### Graceful shutdown or restart

There are a few approaches you can use to perform a graceful shutdown or restart. You can make use of third-party packages specifically built for that, or you can manually do the same with the functions and methods from the built-in packages.

#### Third-party packages

We can use [fvbock/endless](https://github.com/fvbock/endless) to replace the default `ListenAndServe`. Refer to issue [#296](https://github.com/gin-gonic/gin/issues/296) for more details.

```go
router := gin.Default()
router.GET("/", handler)
// [...]
endless.ListenAndServe(":4242", router)
```

Alternatives:

* [grace](https://github.com/facebookgo/grace): Graceful restart & zero downtime deploy for Go servers.
* [graceful](https://github.com/tylerb/graceful): Graceful is a Go package enabling graceful shutdown of an http.Handler server.
* [manners](https://github.com/braintree/manners): A polite Go HTTP server that shuts down gracefully.

#### Manually

In case you are using Go 1.8 or a later version, you may not need to use those libraries. Consider using `http.Server`'s built-in [Shutdown()](https://pkg.go.dev/net/http#Server.Shutdown) method for graceful shutdowns. The example below describes its usage, and we've got more examples using gin [here](https://github.com/gin-gonic/examples/tree/master/graceful-shutdown).

```go
// +build go1.8

package main

import (
  "context"
  "log"
  "net/http"
  "os"
  "os/signal"
  "syscall"
  "time"

  "github.com/gin-gonic/gin"
)

func main() {
  router := gin.Default()
  router.GET("/", func(c *gin.Context) {
    time.Sleep(5 * time.Second)
    c.String(http.StatusOK, "Welcome Gin Server")
  })

  srv := &http.Server{
    Addr:    ":8080",
    Handler: router,
  }

  // Initializing the server in a goroutine so that
  // it won't block the graceful shutdown handling below
  go func() {
    if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
      log.Printf("listen: %s\n", err)
    }
  }()

  // Wait for interrupt signal to gracefully shutdown the server with
  // a timeout of 5 seconds.
  quit := make(chan os.Signal)
  // kill (no param) default send syscall.SIGTERM
  // kill -2 is syscall.SIGINT
  // kill -9 is syscall.SIGKILL but can't be caught, so don't need to add it
  signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
  <-quit
  log.Println("Shutting down server...")

  // The context is used to inform the server it has 5 seconds to finish
  // the request it is currently handling
  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
  defer cancel()

  if err := srv.Shutdown(ctx); err != nil {
    log.Fatal("Server forced to shutdown:", err)
  }

  log.Println("Server exiting")
}
```

### Build a single binary with templates

You can build a server into a single binary containing templates by using the [embed](https://pkg.go.dev/embed) package.

```go
package main

import (
  "embed"
  "html/template"
  "net/http"

  "github.com/gin-gonic/gin"
)

//go:embed assets/* templates/*
var f embed.FS

func main() {
  router := gin.Default()
  templ := template.Must(template.New("").ParseFS(f, "templates/*.tmpl", "templates/foo/*.tmpl"))
  router.SetHTMLTemplate(templ)

  // example: /public/assets/images/example.png
  router.StaticFS("/public", http.FS(f))

  router.GET("/", func(c *gin.Context) {
    c.HTML(http.StatusOK, "index.tmpl", gin.H{
      "title": "Main website",
    })
  })

  router.GET("/foo", func(c *gin.Context) {
    c.HTML(http.StatusOK, "bar.tmpl", gin.H{
      "title": "Foo website",
    })
  })

  router.GET("favicon.ico", func(c *gin.Context) {
    file, _ := f.ReadFile("assets/favicon.ico")
    c.Data(
      http.StatusOK,
      "image/x-icon",
      file,
    )
  })

  router.Run(":8080")
}
```

See a complete example in the `https://github.com/gin-gonic/examples/tree/master/assets-in-binary/example02` directory.

### Bind form-data request with custom struct

The follow example using custom struct:

```go
type StructA struct {
    FieldA string `form:"field_a"`
}

type StructB struct {
    NestedStruct StructA
    FieldB string `form:"field_b"`
}

type StructC struct {
    NestedStructPointer *StructA
    FieldC string `form:"field_c"`
}

type StructD struct {
    NestedAnonyStruct struct {
        FieldX string `form:"field_x"`
    }
    FieldD string `form:"field_d"`
}

func GetDataB(c *gin.Context) {
    var b StructB
    c.Bind(&b)
    c.JSON(http.StatusOK, gin.H{
        "a": b.NestedStruct,
        "b": b.FieldB,
    })
}

func GetDataC(c *gin.Context) {
    var b StructC
    c.Bind(&b)
    c.JSON(http.StatusOK, gin.H{
        "a": b.NestedStructPointer,
        "c": b.FieldC,
    })
}

func GetDataD(c *gin.Context) {
    var b StructD
    c.Bind(&b)
    c.JSON(http.StatusOK, gin.H{
        "x": b.NestedAnonyStruct,
        "d": b.FieldD,
    })
}

func main() {
    r := gin.Default()
    r.GET("/getb", GetDataB)
    r.GET("/getc", GetDataC)
    r.GET("/getd", GetDataD)

    r.Run()
}
```

Using the command `curl` command result:

```sh
$ curl "http://localhost:8080/getb?field_a=hello&field_b=world"
{"a":{"FieldA":"hello"},"b":"world"}
$ curl "http://localhost:8080/getc?field_a=hello&field_c=world"
{"a":{"FieldA":"hello"},"c":"world"}
$ curl "http://localhost:8080/getd?field_x=hello&field_d=world"
{"d":"world","x":{"FieldX":"hello"}}
```

### Try to bind body into different structs

The normal methods for binding request body consumes `c.Request.Body` and they
cannot be called multiple times.

```go
type formA struct {
  Foo string `json:"foo" xml:"foo" binding:"required"`
}

type formB struct {
  Bar string `json:"bar" xml:"bar" binding:"required"`
}

func SomeHandler(c *gin.Context) {
  objA := formA{}
  objB := formB{}
  // Calling c.ShouldBind consumes c.Request.Body and it cannot be reused.
  if errA := c.ShouldBind(&objA); errA == nil {
    c.String(http.StatusOK, `the body should be formA`)
  // Always an error is occurred by this because c.Request.Body is EOF now.
  } else if errB := c.ShouldBind(&objB); errB == nil {
    c.String(http.StatusOK, `the body should be formB`)
  } else {
    ...
  }
}
```

For this, you can use `c.ShouldBindBodyWith` or shortcuts.

- `c.ShouldBindBodyWithJSON` is a shortcut for c.ShouldBindBodyWith(obj, binding.JSON).
- `c.ShouldBindBodyWithXML` is a shortcut for c.ShouldBindBodyWith(obj, binding.XML).
- `c.ShouldBindBodyWithYAML` is a shortcut for c.ShouldBindBodyWith(obj, binding.YAML).
- `c.ShouldBindBodyWithTOML` is a shortcut for c.ShouldBindBodyWith(obj, binding.TOML).

```go
func SomeHandler(c *gin.Context) {
  objA := formA{}
  objB := formB{}
  // This reads c.Request.Body and stores the result into the context.
  if errA := c.ShouldBindBodyWith(&objA, binding.Form); errA == nil {
    c.String(http.StatusOK, `the body should be formA`)
  // At this time, it reuses body stored in the context.
  } else if errB := c.ShouldBindBodyWith(&objB, binding.JSON); errB == nil {
    c.String(http.StatusOK, `the body should be formB JSON`)
  // And it can accepts other formats
  } else if errB2 := c.ShouldBindBodyWithXML(&objB); errB2 == nil {
    c.String(http.StatusOK, `the body should be formB XML`)
  } else {
    ...
  }
}
```

1. `c.ShouldBindBodyWith` stores body into the context before binding. This has
a slight impact to performance, so you should not use this method if you are
enough to call binding at once.
2. This feature is only needed for some formats -- `JSON`, `XML`, `MsgPack`,
`ProtoBuf`. For other formats, `Query`, `Form`, `FormPost`, `FormMultipart`,
can be called by `c.ShouldBind()` multiple times without any damage to
performance (See [#1341](https://github.com/gin-gonic/gin/pull/1341)).

### Bind form-data request with custom struct and custom tag

```go
const (
  customerTag = "url"
  defaultMemory = 32 << 20
)

type customerBinding struct {}

func (customerBinding) Name() string {
  return "form"
}

func (customerBinding) Bind(req *http.Request, obj any) error {
  if err := req.ParseForm(); err != nil {
    return err
  }
  if err := req.ParseMultipartForm(defaultMemory); err != nil {
    if err != http.ErrNotMultipart {
      return err
    }
  }
  if err := binding.MapFormWithTag(obj, req.Form, customerTag); err != nil {
    return err
  }
  return validate(obj)
}

func validate(obj any) error {
  if binding.Validator == nil {
    return nil
  }
  return binding.Validator.ValidateStruct(obj)
}

// Now we can do this!!!
// FormA is an external type that we can't modify it's tag
type FormA struct {
  FieldA string `url:"field_a"`
}

func ListHandler(s *Service) func(ctx *gin.Context) {
  return func(ctx *gin.Context) {
    var urlBinding = customerBinding{}
    var opt FormA
    err := ctx.MustBindWith(&opt, urlBinding)
    if err != nil {
      ...
    }
    ...
  }
}
```

### http2 server push

http.Pusher is supported only **go1.8+**. See the [golang blog](https://go.dev/blog/h2push) for detail information.

```go
package main

import (
  "html/template"
  "log"
  "net/http"

  "github.com/gin-gonic/gin"
)

var html = template.Must(template.New("https").Parse(`
<html>
<head>
  <title>Https Test</title>
  <script src="/assets/app.js"></script>
</head>
<body>
  <h1 style="color:red;">Welcome, Ginner!</h1>
</body>
</html>
`))

func main() {
  r := gin.Default()
  r.Static("/assets", "./assets")
  r.SetHTMLTemplate(html)

  r.GET("/", func(c *gin.Context) {
    if pusher := c.Writer.Pusher(); pusher != nil {
      // use pusher.Push() to do server push
      if err := pusher.Push("/assets/app.js", nil); err != nil {
        log.Printf("Failed to push: %v", err)
      }
    }
    c.HTML(http.StatusOK, "https", gin.H{
      "status": "success",
    })
  })

  // Listen and Server in https://127.0.0.1:8080
  r.RunTLS(":8080", "./testdata/server.pem", "./testdata/server.key")
}
```

### Define format for the log of routes

The default log of routes is:

```sh
[GIN-debug] POST   /foo                      --> main.main.func1 (3 handlers)
[GIN-debug] GET    /bar                      --> main.main.func2 (3 handlers)
[GIN-debug] GET    /status                   --> main.main.func3 (3 handlers)
```

If you want to log this information in given format (e.g. JSON, key values or something else), then you can define this format with `gin.DebugPrintRouteFunc`.
In the example below, we log all routes with standard log package but you can use another log tools that suits of your needs.

```go
import (
  "log"
  "net/http"

  "github.com/gin-gonic/gin"
)

func main() {
  r := gin.Default()
  gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
    log.Printf("endpoint %v %v %v %v\n", httpMethod, absolutePath, handlerName, nuHandlers)
  }

  r.POST("/foo", func(c *gin.Context) {
    c.JSON(http.StatusOK, "foo")
  })

  r.GET("/bar", func(c *gin.Context) {
    c.JSON(http.StatusOK, "bar")
  })

  r.GET("/status", func(c *gin.Context) {
    c.JSON(http.StatusOK, "ok")
  })

  // Listen and Server in http://0.0.0.0:8080
  r.Run()
}
```

### Set and get a cookie

```go
import (
  "fmt"

  "github.com/gin-gonic/gin"
)

func main() {
  router := gin.Default()

  router.GET("/cookie", func(c *gin.Context) {
    cookie, err := c.Cookie("gin_cookie")

    if err != nil {
      cookie = "NotSet"
      // Using http.Cookie struct for more control
      c.SetCookieData(&http.Cookie{
        Name:       "gin_cookie",
        Value:      "test",
        Path:       "/",
        Domain:     "localhost",
        MaxAge:     3600,
        Secure:     false,
        HttpOnly:   true,
        // Additional fields available in http.Cookie
        Expires:    time.Now().Add(24 * time.Hour),
        // Partitioned: true, // Available in newer Go versions
      })
    }

    fmt.Printf("Cookie value: %s \n", cookie)
  })

  router.Run()
}
```

You can also use the `SetCookieData` method, which accepts a `*http.Cookie` directly for more flexibility:

```go
import (
  "fmt"
  "net/http"
  "time"

  "github.com/gin-gonic/gin"
)

func main() {
  router := gin.Default()

  router.GET("/cookie", func(c *gin.Context) {
      cookie, err := c.Cookie("gin_cookie")

      if err != nil {
          cookie = "NotSet"
          // Using http.Cookie struct for more control
          c.SetCookieData(&http.Cookie{
              Name:       "gin_cookie",
              Value:      "test",
              Path:       "/",
              Domain:     "localhost",
              MaxAge:     3600,
              Secure:     false,
              HttpOnly:   true,
              // Additional fields available in http.Cookie
              Expires:    time.Now().Add(24 * time.Hour),
              // Partitioned: true, // Available in newer Go versions
          })
      }

      fmt.Printf("Cookie value: %s \n", cookie)
  })

  router.Run()
}
```

### Custom json codec at runtime

Gin support custom json serialization and deserialization logic without using compile tags.

1. Define a custom struct implements the `json.Core` interface.

2. Before your engine starts, assign values to `json.API` using the custom struct.

```go
package main

import (
  "io"

  "github.com/gin-gonic/gin"
  "github.com/gin-gonic/gin/codec/json"
  jsoniter "github.com/json-iterator/go"
)

var customConfig = jsoniter.Config{
  EscapeHTML:             true,
  SortMapKeys:            true,
  ValidateJsonRawMessage: true,
}.Froze()

// implement api.JsonApi
type customJsonApi struct {
}

func (j customJsonApi) Marshal(v any) ([]byte, error) {
  return customConfig.Marshal(v)
}

func (j customJsonApi) Unmarshal(data []byte, v any) error {
  return customConfig.Unmarshal(data, v)
}

func (j customJsonApi) MarshalIndent(v any, prefix, indent string) ([]byte, error) {
  return customConfig.MarshalIndent(v, prefix, indent)
}

func (j customJsonApi) NewEncoder(writer io.Writer) json.Encoder {
  return customConfig.NewEncoder(writer)
}

func (j customJsonApi) NewDecoder(reader io.Reader) json.Decoder {
  return customConfig.NewDecoder(reader)
}

func main() {
  //Replace the default json api
  json.API = customJsonApi{}

  //Start your gin engine
  router := gin.Default()
  router.Run(":8080")
}
```

## Don't trust all proxies

Gin lets you specify which headers to hold the real client IP (if any),
as well as specifying which proxies (or direct clients) you trust to
specify one of these headers.

Use function `SetTrustedProxies()` on your `gin.Engine` to specify network addresses
or network CIDRs from where clients which their request headers related to client
IP can be trusted. They can be IPv4 addresses, IPv4 CIDRs, IPv6 addresses or
IPv6 CIDRs.

**Attention:** Gin trusts all proxies by default if you don't specify a trusted
proxy using the function above, **this is NOT safe**. At the same time, if you don't
use any proxy, you can disable this feature by using `Engine.SetTrustedProxies(nil)`,
then `Context.ClientIP()` will return the remote address directly to avoid some
unnecessary computation.

```go
import (
  "fmt"

  "github.com/gin-gonic/gin"
)

func main() {
  router := gin.Default()
  router.SetTrustedProxies([]string{"192.168.1.2"})

  router.GET("/", func(c *gin.Context) {
    // If the client is 192.168.1.2, use the X-Forwarded-For
    // header to deduce the original client IP from the trust-
    // worthy parts of that header.
    // Otherwise, simply return the direct client IP
    fmt.Printf("ClientIP: %s\n", c.ClientIP())
  })
  router.Run()
}
```

**Notice:** If you are using a CDN service, you can set the `Engine.TrustedPlatform`
to skip TrustedProxies check, it has a higher priority than TrustedProxies.
Look at the example below:

```go
import (
  "fmt"

  "github.com/gin-gonic/gin"
)

func main() {
  router := gin.Default()
  // Use predefined header gin.PlatformXXX
  // Google App Engine
  router.TrustedPlatform = gin.PlatformGoogleAppEngine
  // Cloudflare
  router.TrustedPlatform = gin.PlatformCloudflare
  // Fly.io
  router.TrustedPlatform = gin.PlatformFlyIO
  // Or, you can set your own trusted request header. But be sure your CDN
  // prevents users from passing this header! For example, if your CDN puts
  // the client IP in X-CDN-Client-IP:
  router.TrustedPlatform = "X-CDN-Client-IP"

  router.GET("/", func(c *gin.Context) {
    // If you set TrustedPlatform, ClientIP() will resolve the
    // corresponding header and return IP directly
    fmt.Printf("ClientIP: %s\n", c.ClientIP())
  })
  router.Run()
}
```

## Testing

The `net/http/httptest` package is preferable way for HTTP testing.

```go
package main

import (
  "net/http"

  "github.com/gin-gonic/gin"
)

func setupRouter() *gin.Engine {
  r := gin.Default()
  r.GET("/ping", func(c *gin.Context) {
    c.String(http.StatusOK, "pong")
  })
  return r
}

func main() {
  r := setupRouter()
  r.Run(":8080")
}
```

Test for code example above:

```go
package main

import (
  "net/http"
  "net/http/httptest"
  "testing"

  "github.com/stretchr/testify/assert"
)

func TestPingRoute(t *testing.T) {
  router := setupRouter()

  w := httptest.NewRecorder()
  req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
  router.ServeHTTP(w, req)

  assert.Equal(t, http.StatusOK, w.Code)
  assert.Equal(t, "pong", w.Body.String())
}
```

## BENCHMARKS.md

# Benchmark System

**VM HOST:** Travis
**Machine:** Ubuntu 16.04.6 LTS x64
**Date:** May 04th, 2020
**Version:** Gin v1.6.3
**Go Version:** 1.14.2 linux/amd64
**Source:** [Go HTTP Router Benchmark](https://github.com/gin-gonic/go-http-routing-benchmark)
**Result:** [See the gist](https://gist.github.com/appleboy/b5f2ecfaf50824ae9c64dcfb9165ae5e) or [Travis result](https://travis-ci.org/github/gin-gonic/go-http-routing-benchmark/jobs/682947061)

## Static Routes: 157

```sh
Gin: 34936 Bytes

HttpServeMux: 14512 Bytes
Ace: 30680 Bytes
Aero: 34536 Bytes
Bear: 30456 Bytes
Beego: 98456 Bytes
Bone: 40224 Bytes
Chi: 83608 Bytes
Denco: 10216 Bytes
Echo: 80328 Bytes
GocraftWeb: 55288 Bytes
Goji: 29744 Bytes
Gojiv2: 105840 Bytes
GoJsonRest: 137496 Bytes
GoRestful: 816936 Bytes
GorillaMux: 585632 Bytes
GowwwRouter: 24968 Bytes
HttpRouter: 21712 Bytes
HttpTreeMux: 73448 Bytes
Kocha: 115472 Bytes
LARS: 30640 Bytes
Macaron: 38592 Bytes
Martini: 310864 Bytes
Pat: 19696 Bytes
Possum: 89920 Bytes
R2router: 23712 Bytes
Rivet: 24608 Bytes
Tango: 28264 Bytes
TigerTonic: 78768 Bytes
Traffic: 538976 Bytes
Vulcan: 369960 Bytes
```

## GithubAPI Routes: 203

```sh
Gin: 58512 Bytes

Ace: 48688 Bytes
Aero: 318568 Bytes
Bear: 84248 Bytes
Beego: 150936 Bytes
Bone: 100976 Bytes
Chi: 95112 Bytes
Denco: 36736 Bytes
Echo: 100296 Bytes
GocraftWeb: 95432 Bytes
Goji: 49680 Bytes
Gojiv2: 104704 Bytes
GoJsonRest: 141976 Bytes
GoRestful: 1241656 Bytes
GorillaMux: 1322784 Bytes
GowwwRouter: 80008 Bytes
HttpRouter: 37144 Bytes
HttpTreeMux: 78800 Bytes
Kocha: 785120 Bytes
LARS: 48600 Bytes
Macaron: 92784 Bytes
Martini: 485264 Bytes
Pat: 21200 Bytes
Possum: 85312 Bytes
R2router: 47104 Bytes
Rivet: 42840 Bytes
Tango: 54840 Bytes
TigerTonic: 95264 Bytes
Traffic: 921744 Bytes
Vulcan: 425992 Bytes
```

## GPlusAPI Routes: 13

```sh
Gin: 4384 Bytes

Ace: 3712 Bytes
Aero: 26056 Bytes
Bear: 7112 Bytes
Beego: 10272 Bytes
Bone: 6688 Bytes
Chi: 8024 Bytes
Denco: 3264 Bytes
Echo: 9688 Bytes
GocraftWeb: 7496 Bytes
Goji: 3152 Bytes
Gojiv2: 7376 Bytes
GoJsonRest: 11400 Bytes
GoRestful: 74328 Bytes
GorillaMux: 66208 Bytes
GowwwRouter: 5744 Bytes
HttpRouter: 2808 Bytes
HttpTreeMux: 7440 Bytes
Kocha: 128880 Bytes
LARS: 3656 Bytes
Macaron: 8656 Bytes
Martini: 23920 Bytes
Pat: 1856 Bytes
Possum: 7248 Bytes
R2router: 3928 Bytes
Rivet: 3064 Bytes
Tango: 5168 Bytes
TigerTonic: 9408 Bytes
Traffic: 46400 Bytes
Vulcan: 25544 Bytes
```

## ParseAPI Routes: 26

```sh
Gin: 7776 Bytes

Ace: 6704 Bytes
Aero: 28488 Bytes
Bear: 12320 Bytes
Beego: 19280 Bytes
Bone: 11440 Bytes
Chi: 9744 Bytes
Denco: 4192 Bytes
Echo: 11664 Bytes
GocraftWeb: 12800 Bytes
Goji: 5680 Bytes
Gojiv2: 14464 Bytes
GoJsonRest: 14072 Bytes
GoRestful: 116264 Bytes
GorillaMux: 105880 Bytes
GowwwRouter: 9344 Bytes
HttpRouter: 5072 Bytes
HttpTreeMux: 7848 Bytes
Kocha: 181712 Bytes
LARS: 6632 Bytes
Macaron: 13648 Bytes
Martini: 45888 Bytes
Pat: 2560 Bytes
Possum: 9200 Bytes
R2router: 7056 Bytes
Rivet: 5680 Bytes
Tango: 8920 Bytes
TigerTonic: 9840 Bytes
Traffic: 79096 Bytes
Vulcan: 44504 Bytes
```

## Static Routes

```sh
BenchmarkGin_StaticAll                   62169         19319 ns/op           0 B/op           0 allocs/op

BenchmarkAce_StaticAll                   65428         18313 ns/op           0 B/op           0 allocs/op
BenchmarkAero_StaticAll                 121132          9632 ns/op           0 B/op           0 allocs/op
BenchmarkHttpServeMux_StaticAll          52626         22758 ns/op           0 B/op           0 allocs/op
BenchmarkBeego_StaticAll                  9962        179058 ns/op       55264 B/op         471 allocs/op
BenchmarkBear_StaticAll                  14894         80966 ns/op       20272 B/op         469 allocs/op
BenchmarkBone_StaticAll                  18718         64065 ns/op           0 B/op           0 allocs/op
BenchmarkChi_StaticAll                   10000        149827 ns/op       67824 B/op         471 allocs/op
BenchmarkDenco_StaticAll                211393          5680 ns/op           0 B/op           0 allocs/op
BenchmarkEcho_StaticAll                  49341         24343 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_StaticAll            10000        126209 ns/op       46312 B/op         785 allocs/op
BenchmarkGoji_StaticAll                  27956         43174 ns/op           0 B/op           0 allocs/op
BenchmarkGojiv2_StaticAll                 3430        370718 ns/op      205984 B/op        1570 allocs/op
BenchmarkGoJsonRest_StaticAll             9134        188888 ns/op       51653 B/op        1727 allocs/op
BenchmarkGoRestful_StaticAll               706       1703330 ns/op      613280 B/op        2053 allocs/op
BenchmarkGorillaMux_StaticAll             1268        924083 ns/op      153233 B/op        1413 allocs/op
BenchmarkGowwwRouter_StaticAll           63374         18935 ns/op           0 B/op           0 allocs/op
BenchmarkHttpRouter_StaticAll           109938         10902 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_StaticAll          109166         10861 ns/op           0 B/op           0 allocs/op
BenchmarkKocha_StaticAll                 92258         12992 ns/op           0 B/op           0 allocs/op
BenchmarkLARS_StaticAll                  65200         18387 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_StaticAll                5671        291501 ns/op      115553 B/op        1256 allocs/op
BenchmarkMartini_StaticAll                 807       1460498 ns/op      125444 B/op        1717 allocs/op
BenchmarkPat_StaticAll                     513       2342396 ns/op      602832 B/op       12559 allocs/op
BenchmarkPossum_StaticAll                10000        128270 ns/op       65312 B/op         471 allocs/op
BenchmarkR2router_StaticAll              16726         71760 ns/op       22608 B/op         628 allocs/op
BenchmarkRivet_StaticAll                 41722         28723 ns/op           0 B/op           0 allocs/op
BenchmarkTango_StaticAll                  7606        205082 ns/op       39209 B/op        1256 allocs/op
BenchmarkTigerTonic_StaticAll            26247         45806 ns/op        7376 B/op         157 allocs/op
BenchmarkTraffic_StaticAll                 550       2284518 ns/op      754864 B/op       14601 allocs/op
BenchmarkVulcan_StaticAll                10000        131343 ns/op       15386 B/op         471 allocs/op
```

## Micro Benchmarks

```sh
BenchmarkGin_Param                    18785022          63.9 ns/op           0 B/op           0 allocs/op

BenchmarkAce_Param                    14689765          81.5 ns/op           0 B/op           0 allocs/op
BenchmarkAero_Param                   23094770          51.2 ns/op           0 B/op           0 allocs/op
BenchmarkBear_Param                    1417045           845 ns/op         456 B/op           5 allocs/op
BenchmarkBeego_Param                   1000000          1080 ns/op         352 B/op           3 allocs/op
BenchmarkBone_Param                    1000000          1463 ns/op         816 B/op           6 allocs/op
BenchmarkChi_Param                     1378756           885 ns/op         432 B/op           3 allocs/op
BenchmarkDenco_Param                   8557899           143 ns/op          32 B/op           1 allocs/op
BenchmarkEcho_Param                   16433347          75.5 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_Param              1000000          1218 ns/op         648 B/op           8 allocs/op
BenchmarkGoji_Param                    1921248           617 ns/op         336 B/op           2 allocs/op
BenchmarkGojiv2_Param                   561848          2156 ns/op        1328 B/op          11 allocs/op
BenchmarkGoJsonRest_Param              1000000          1358 ns/op         649 B/op          13 allocs/op
BenchmarkGoRestful_Param                224857          5307 ns/op        4192 B/op          14 allocs/op
BenchmarkGorillaMux_Param               498313          2459 ns/op        1280 B/op          10 allocs/op
BenchmarkGowwwRouter_Param             1864354           654 ns/op         432 B/op           3 allocs/op
BenchmarkHttpRouter_Param             26269074          47.7 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_Param             2109829           557 ns/op         352 B/op           3 allocs/op
BenchmarkKocha_Param                   5050216           243 ns/op          56 B/op           3 allocs/op
BenchmarkLARS_Param                   19811712          59.9 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_Param                  662746          2329 ns/op        1072 B/op          10 allocs/op
BenchmarkMartini_Param                  279902          4260 ns/op        1072 B/op          10 allocs/op
BenchmarkPat_Param                     1000000          1382 ns/op         536 B/op          11 allocs/op
BenchmarkPossum_Param                  1000000          1014 ns/op         496 B/op           5 allocs/op
BenchmarkR2router_Param                1712559           707 ns/op         432 B/op           5 allocs/op
BenchmarkRivet_Param                   6648086           182 ns/op          48 B/op           1 allocs/op
BenchmarkTango_Param                   1221504           994 ns/op         248 B/op           8 allocs/op
BenchmarkTigerTonic_Param               891661          2261 ns/op         776 B/op          16 allocs/op
BenchmarkTraffic_Param                  350059          3598 ns/op        1856 B/op          21 allocs/op
BenchmarkVulcan_Param                  2517823           472 ns/op          98 B/op           3 allocs/op
BenchmarkAce_Param5                    9214365           130 ns/op           0 B/op           0 allocs/op
BenchmarkAero_Param5                  15369013          77.9 ns/op           0 B/op           0 allocs/op
BenchmarkBear_Param5                   1000000          1113 ns/op         501 B/op           5 allocs/op
BenchmarkBeego_Param5                  1000000          1269 ns/op         352 B/op           3 allocs/op
BenchmarkBone_Param5                    986820          1873 ns/op         864 B/op           6 allocs/op
BenchmarkChi_Param5                    1000000          1156 ns/op         432 B/op           3 allocs/op
BenchmarkDenco_Param5                  3036331           400 ns/op         160 B/op           1 allocs/op
BenchmarkEcho_Param5                   6447133           186 ns/op           0 B/op           0 allocs/op
BenchmarkGin_Param5                   10786068           110 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_Param5              844820          1944 ns/op         920 B/op          11 allocs/op
BenchmarkGoji_Param5                   1474965           827 ns/op         336 B/op           2 allocs/op
BenchmarkGojiv2_Param5                  442820          2516 ns/op        1392 B/op          11 allocs/op
BenchmarkGoJsonRest_Param5              507555          2711 ns/op        1097 B/op          16 allocs/op
BenchmarkGoRestful_Param5               216481          6093 ns/op        4288 B/op          14 allocs/op
BenchmarkGorillaMux_Param5              314402          3628 ns/op        1344 B/op          10 allocs/op
BenchmarkGowwwRouter_Param5            1624660           733 ns/op         432 B/op           3 allocs/op
BenchmarkHttpRouter_Param5            13167324          92.0 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_Param5            1000000          1295 ns/op         576 B/op           6 allocs/op
BenchmarkKocha_Param5                  1000000          1138 ns/op         440 B/op          10 allocs/op
BenchmarkLARS_Param5                  11580613           105 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_Param5                 473596          2755 ns/op        1072 B/op          10 allocs/op
BenchmarkMartini_Param5                 230756          5111 ns/op        1232 B/op          11 allocs/op
BenchmarkPat_Param5                     469190          3370 ns/op         888 B/op          29 allocs/op
BenchmarkPossum_Param5                 1000000          1002 ns/op         496 B/op           5 allocs/op
BenchmarkR2router_Param5               1422129           844 ns/op         432 B/op           5 allocs/op
BenchmarkRivet_Param5                  2263789           539 ns/op         240 B/op           1 allocs/op
BenchmarkTango_Param5                  1000000          1256 ns/op         360 B/op           8 allocs/op
BenchmarkTigerTonic_Param5              175500          7492 ns/op        2279 B/op          39 allocs/op
BenchmarkTraffic_Param5                 233631          5816 ns/op        2208 B/op          27 allocs/op
BenchmarkVulcan_Param5                 1923416           629 ns/op          98 B/op           3 allocs/op
BenchmarkAce_Param20                   4321266           281 ns/op           0 B/op           0 allocs/op
BenchmarkAero_Param20                 31501641          35.2 ns/op           0 B/op           0 allocs/op
BenchmarkBear_Param20                   335204          3489 ns/op        1665 B/op           5 allocs/op
BenchmarkBeego_Param20                  503674          2860 ns/op         352 B/op           3 allocs/op
BenchmarkBone_Param20                   298922          4741 ns/op        2031 B/op           6 allocs/op
BenchmarkChi_Param20                    878181          1957 ns/op         432 B/op           3 allocs/op
BenchmarkDenco_Param20                 1000000          1360 ns/op         640 B/op           1 allocs/op
BenchmarkEcho_Param20                  2104946           580 ns/op           0 B/op           0 allocs/op
BenchmarkGin_Param20                   4167204           290 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_Param20             173064          7514 ns/op        3796 B/op          15 allocs/op
BenchmarkGoji_Param20                   458778          2651 ns/op        1247 B/op           2 allocs/op
BenchmarkGojiv2_Param20                 364862          3178 ns/op        1632 B/op          11 allocs/op
BenchmarkGoJsonRest_Param20             125514          9760 ns/op        4485 B/op          20 allocs/op
BenchmarkGoRestful_Param20              101217         11964 ns/op        6715 B/op          18 allocs/op
BenchmarkGorillaMux_Param20             147654          8132 ns/op        3452 B/op          12 allocs/op
BenchmarkGowwwRouter_Param20           1000000          1225 ns/op         432 B/op           3 allocs/op
BenchmarkHttpRouter_Param20            4920895           247 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_Param20            173202          6605 ns/op        3196 B/op          10 allocs/op
BenchmarkKocha_Param20                  345988          3620 ns/op        1808 B/op          27 allocs/op
BenchmarkLARS_Param20                  4592326           262 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_Param20                166492          7286 ns/op        2924 B/op          12 allocs/op
BenchmarkMartini_Param20                122162         10653 ns/op        3595 B/op          13 allocs/op
BenchmarkPat_Param20                     78630         15239 ns/op        4424 B/op          93 allocs/op
BenchmarkPossum_Param20                1000000          1008 ns/op         496 B/op           5 allocs/op
BenchmarkR2router_Param20               294981          4587 ns/op        2284 B/op           7 allocs/op
BenchmarkRivet_Param20                  691798          2090 ns/op        1024 B/op           1 allocs/op
BenchmarkTango_Param20                  842440          2505 ns/op         856 B/op           8 allocs/op
BenchmarkTigerTonic_Param20              38614         31509 ns/op        9870 B/op         119 allocs/op
BenchmarkTraffic_Param20                 57633         21107 ns/op        7853 B/op          47 allocs/op
BenchmarkVulcan_Param20                1000000          1178 ns/op          98 B/op           3 allocs/op
BenchmarkAce_ParamWrite                7330743           180 ns/op           8 B/op           1 allocs/op
BenchmarkAero_ParamWrite              13833598          86.7 ns/op           0 B/op           0 allocs/op
BenchmarkBear_ParamWrite               1363321           867 ns/op         456 B/op           5 allocs/op
BenchmarkBeego_ParamWrite              1000000          1104 ns/op         360 B/op           4 allocs/op
BenchmarkBone_ParamWrite               1000000          1475 ns/op         816 B/op           6 allocs/op
BenchmarkChi_ParamWrite                1320590           892 ns/op         432 B/op           3 allocs/op
BenchmarkDenco_ParamWrite              7093605           172 ns/op          32 B/op           1 allocs/op
BenchmarkEcho_ParamWrite               8434424           161 ns/op           8 B/op           1 allocs/op
BenchmarkGin_ParamWrite               10377034           118 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_ParamWrite         1000000          1266 ns/op         656 B/op           9 allocs/op
BenchmarkGoji_ParamWrite               1874168           654 ns/op         336 B/op           2 allocs/op
BenchmarkGojiv2_ParamWrite              459032          2352 ns/op        1360 B/op          13 allocs/op
BenchmarkGoJsonRest_ParamWrite          499434          2145 ns/op        1128 B/op          18 allocs/op
BenchmarkGoRestful_ParamWrite           241087          5470 ns/op        4200 B/op          15 allocs/op
BenchmarkGorillaMux_ParamWrite          425686          2522 ns/op        1280 B/op          10 allocs/op
BenchmarkGowwwRouter_ParamWrite         922172          1778 ns/op         976 B/op           8 allocs/op
BenchmarkHttpRouter_ParamWrite        15392049          77.7 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_ParamWrite        1973385           597 ns/op         352 B/op           3 allocs/op
BenchmarkKocha_ParamWrite              4262500           281 ns/op          56 B/op           3 allocs/op
BenchmarkLARS_ParamWrite              10764410           113 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_ParamWrite             486769          2726 ns/op        1176 B/op          14 allocs/op
BenchmarkMartini_ParamWrite             264804          4842 ns/op        1176 B/op          14 allocs/op
BenchmarkPat_ParamWrite                 735116          2047 ns/op         960 B/op          15 allocs/op
BenchmarkPossum_ParamWrite             1000000          1004 ns/op         496 B/op           5 allocs/op
BenchmarkR2router_ParamWrite           1592136           768 ns/op         432 B/op           5 allocs/op
BenchmarkRivet_ParamWrite              3582051           339 ns/op         112 B/op           2 allocs/op
BenchmarkTango_ParamWrite              2237337           534 ns/op         136 B/op           4 allocs/op
BenchmarkTigerTonic_ParamWrite          439608          3136 ns/op        1216 B/op          21 allocs/op
BenchmarkTraffic_ParamWrite             306979          4328 ns/op        2280 B/op          25 allocs/op
BenchmarkVulcan_ParamWrite             2529973           472 ns/op          98 B/op           3 allocs/op
```

## GitHub

```sh
BenchmarkGin_GithubStatic             15629472          76.7 ns/op           0 B/op           0 allocs/op

BenchmarkAce_GithubStatic             15542612          75.9 ns/op           0 B/op           0 allocs/op
BenchmarkAero_GithubStatic            24777151          48.5 ns/op           0 B/op           0 allocs/op
BenchmarkBear_GithubStatic             2788894           435 ns/op         120 B/op           3 allocs/op
BenchmarkBeego_GithubStatic            1000000          1064 ns/op         352 B/op           3 allocs/op
BenchmarkBone_GithubStatic               93507         12838 ns/op        2880 B/op          60 allocs/op
BenchmarkChi_GithubStatic              1387743           860 ns/op         432 B/op           3 allocs/op
BenchmarkDenco_GithubStatic           39384996          30.4 ns/op           0 B/op           0 allocs/op
BenchmarkEcho_GithubStatic            12076382          99.1 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_GithubStatic       1596495           756 ns/op         296 B/op           5 allocs/op
BenchmarkGoji_GithubStatic             6364876           189 ns/op           0 B/op           0 allocs/op
BenchmarkGojiv2_GithubStatic            550202          2098 ns/op        1312 B/op          10 allocs/op
BenchmarkGoRestful_GithubStatic         102183         12552 ns/op        4256 B/op          13 allocs/op
BenchmarkGoJsonRest_GithubStatic       1000000          1029 ns/op         329 B/op          11 allocs/op
BenchmarkGorillaMux_GithubStatic        255552          5190 ns/op         976 B/op           9 allocs/op
BenchmarkGowwwRouter_GithubStatic     15531916          77.1 ns/op           0 B/op           0 allocs/op
BenchmarkHttpRouter_GithubStatic      27920724          43.1 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_GithubStatic     21448953          55.8 ns/op           0 B/op           0 allocs/op
BenchmarkKocha_GithubStatic           21405310          56.0 ns/op           0 B/op           0 allocs/op
BenchmarkLARS_GithubStatic            13625156          89.0 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_GithubStatic          1000000          1747 ns/op         736 B/op           8 allocs/op
BenchmarkMartini_GithubStatic           187186          7326 ns/op         768 B/op           9 allocs/op
BenchmarkPat_GithubStatic               109143         11563 ns/op        3648 B/op          76 allocs/op
BenchmarkPossum_GithubStatic           1575898           770 ns/op         416 B/op           3 allocs/op
BenchmarkR2router_GithubStatic         3046231           404 ns/op         144 B/op           4 allocs/op
BenchmarkRivet_GithubStatic           11484826           105 ns/op           0 B/op           0 allocs/op
BenchmarkTango_GithubStatic            1000000          1153 ns/op         248 B/op           8 allocs/op
BenchmarkTigerTonic_GithubStatic       4929780           249 ns/op          48 B/op           1 allocs/op
BenchmarkTraffic_GithubStatic           106351         11819 ns/op        4664 B/op          90 allocs/op
BenchmarkVulcan_GithubStatic           1613271           722 ns/op          98 B/op           3 allocs/op
BenchmarkAce_GithubParam               8386032           143 ns/op           0 B/op           0 allocs/op
BenchmarkAero_GithubParam             11816200           102 ns/op           0 B/op           0 allocs/op
BenchmarkBear_GithubParam              1000000          1012 ns/op         496 B/op           5 allocs/op
BenchmarkBeego_GithubParam             1000000          1157 ns/op         352 B/op           3 allocs/op
BenchmarkBone_GithubParam               184653          6912 ns/op        1888 B/op          19 allocs/op
BenchmarkChi_GithubParam               1000000          1102 ns/op         432 B/op           3 allocs/op
BenchmarkDenco_GithubParam             3484798           352 ns/op         128 B/op           1 allocs/op
BenchmarkEcho_GithubParam              6337380           189 ns/op           0 B/op           0 allocs/op
BenchmarkGin_GithubParam               9132032           131 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_GithubParam        1000000          1446 ns/op         712 B/op           9 allocs/op
BenchmarkGoji_GithubParam              1248640           977 ns/op         336 B/op           2 allocs/op
BenchmarkGojiv2_GithubParam             383233          2784 ns/op        1408 B/op          13 allocs/op
BenchmarkGoJsonRest_GithubParam        1000000          1991 ns/op         713 B/op          14 allocs/op
BenchmarkGoRestful_GithubParam           76414         16015 ns/op        4352 B/op          16 allocs/op
BenchmarkGorillaMux_GithubParam         150026          7663 ns/op        1296 B/op          10 allocs/op
BenchmarkGowwwRouter_GithubParam       1592044           751 ns/op         432 B/op           3 allocs/op
BenchmarkHttpRouter_GithubParam       10420628           115 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_GithubParam       1403755           835 ns/op         384 B/op           4 allocs/op
BenchmarkKocha_GithubParam             2286170           533 ns/op         128 B/op           5 allocs/op
BenchmarkLARS_GithubParam              9540374           129 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_GithubParam            533154          2742 ns/op        1072 B/op          10 allocs/op
BenchmarkMartini_GithubParam            119397          9638 ns/op        1152 B/op          11 allocs/op
BenchmarkPat_GithubParam                150675          8858 ns/op        2408 B/op          48 allocs/op
BenchmarkPossum_GithubParam            1000000          1001 ns/op         496 B/op           5 allocs/op
BenchmarkR2router_GithubParam          1602886           761 ns/op         432 B/op           5 allocs/op
BenchmarkRivet_GithubParam             2986579           409 ns/op          96 B/op           1 allocs/op
BenchmarkTango_GithubParam             1000000          1356 ns/op         344 B/op           8 allocs/op
BenchmarkTigerTonic_GithubParam         388899          3429 ns/op        1176 B/op          22 allocs/op
BenchmarkTraffic_GithubParam            123160          9734 ns/op        2816 B/op          40 allocs/op
BenchmarkVulcan_GithubParam            1000000          1138 ns/op          98 B/op           3 allocs/op
BenchmarkAce_GithubAll                   40543         29670 ns/op           0 B/op           0 allocs/op
BenchmarkAero_GithubAll                  57632         20648 ns/op           0 B/op           0 allocs/op
BenchmarkBear_GithubAll                   9234        216179 ns/op       86448 B/op         943 allocs/op
BenchmarkBeego_GithubAll                  7407        243496 ns/op       71456 B/op         609 allocs/op
BenchmarkBone_GithubAll                    420       2922835 ns/op      720160 B/op        8620 allocs/op
BenchmarkChi_GithubAll                    7620        238331 ns/op       87696 B/op         609 allocs/op
BenchmarkDenco_GithubAll                 18355         64494 ns/op       20224 B/op         167 allocs/op
BenchmarkEcho_GithubAll                  31251         38479 ns/op           0 B/op           0 allocs/op
BenchmarkGin_GithubAll                   43550         27364 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_GithubAll             4117        300062 ns/op      131656 B/op        1686 allocs/op
BenchmarkGoji_GithubAll                   3274        416158 ns/op       56112 B/op         334 allocs/op
BenchmarkGojiv2_GithubAll                 1402        870518 ns/op      352720 B/op        4321 allocs/op
BenchmarkGoJsonRest_GithubAll             2976        401507 ns/op      134371 B/op        2737 allocs/op
BenchmarkGoRestful_GithubAll               410       2913158 ns/op      910144 B/op        2938 allocs/op
BenchmarkGorillaMux_GithubAll              346       3384987 ns/op      251650 B/op        1994 allocs/op
BenchmarkGowwwRouter_GithubAll           10000        143025 ns/op       72144 B/op         501 allocs/op
BenchmarkHttpRouter_GithubAll            55938         21360 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_GithubAll           10000        153944 ns/op       65856 B/op         671 allocs/op
BenchmarkKocha_GithubAll                 10000        106315 ns/op       23304 B/op         843 allocs/op
BenchmarkLARS_GithubAll                  47779         25084 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_GithubAll                3266        371907 ns/op      149409 B/op        1624 allocs/op
BenchmarkMartini_GithubAll                 331       3444706 ns/op      226551 B/op        2325 allocs/op
BenchmarkPat_GithubAll                     273       4381818 ns/op     1483152 B/op       26963 allocs/op
BenchmarkPossum_GithubAll                10000        164367 ns/op       84448 B/op         609 allocs/op
BenchmarkR2router_GithubAll              10000        160220 ns/op       77328 B/op         979 allocs/op
BenchmarkRivet_GithubAll                 14625         82453 ns/op       16272 B/op         167 allocs/op
BenchmarkTango_GithubAll                  6255        279611 ns/op       63826 B/op        1618 allocs/op
BenchmarkTigerTonic_GithubAll             2008        687874 ns/op      193856 B/op        4474 allocs/op
BenchmarkTraffic_GithubAll                 355       3478508 ns/op      820744 B/op       14114 allocs/op
BenchmarkVulcan_GithubAll                 6885        193333 ns/op       19894 B/op         609 allocs/op
```

## Google+

```sh
BenchmarkGin_GPlusStatic              19247326          62.2 ns/op           0 B/op           0 allocs/op

BenchmarkAce_GPlusStatic              20235060          59.2 ns/op           0 B/op           0 allocs/op
BenchmarkAero_GPlusStatic             31978935          37.6 ns/op           0 B/op           0 allocs/op
BenchmarkBear_GPlusStatic              3516523           341 ns/op         104 B/op           3 allocs/op
BenchmarkBeego_GPlusStatic             1212036           991 ns/op         352 B/op           3 allocs/op
BenchmarkBone_GPlusStatic              6736242           183 ns/op          32 B/op           1 allocs/op
BenchmarkChi_GPlusStatic               1490640           814 ns/op         432 B/op           3 allocs/op
BenchmarkDenco_GPlusStatic            55006856          21.8 ns/op           0 B/op           0 allocs/op
BenchmarkEcho_GPlusStatic             17688258          67.9 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_GPlusStatic        1829181           666 ns/op         280 B/op           5 allocs/op
BenchmarkGoji_GPlusStatic              9147451           130 ns/op           0 B/op           0 allocs/op
BenchmarkGojiv2_GPlusStatic             594015          2063 ns/op        1312 B/op          10 allocs/op
BenchmarkGoJsonRest_GPlusStatic        1264906           950 ns/op         329 B/op          11 allocs/op
BenchmarkGoRestful_GPlusStatic          231558          5341 ns/op        3872 B/op          13 allocs/op
BenchmarkGorillaMux_GPlusStatic         908418          1809 ns/op         976 B/op           9 allocs/op
BenchmarkGowwwRouter_GPlusStatic      40684604          29.5 ns/op           0 B/op           0 allocs/op
BenchmarkHttpRouter_GPlusStatic       46742804          25.7 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_GPlusStatic      32567161          36.9 ns/op           0 B/op           0 allocs/op
BenchmarkKocha_GPlusStatic            33800060          35.3 ns/op           0 B/op           0 allocs/op
BenchmarkLARS_GPlusStatic             20431858          60.0 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_GPlusStatic           1000000          1745 ns/op         736 B/op           8 allocs/op
BenchmarkMartini_GPlusStatic            442248          3619 ns/op         768 B/op           9 allocs/op
BenchmarkPat_GPlusStatic               4328004           292 ns/op          96 B/op           2 allocs/op
BenchmarkPossum_GPlusStatic            1570753           763 ns/op         416 B/op           3 allocs/op
BenchmarkR2router_GPlusStatic          3339474           355 ns/op         144 B/op           4 allocs/op
BenchmarkRivet_GPlusStatic            18570961          64.7 ns/op           0 B/op           0 allocs/op
BenchmarkTango_GPlusStatic             1388702           860 ns/op         200 B/op           8 allocs/op
BenchmarkTigerTonic_GPlusStatic        7803543           159 ns/op          32 B/op           1 allocs/op
BenchmarkTraffic_GPlusStatic            878605          2171 ns/op        1112 B/op          16 allocs/op
BenchmarkVulcan_GPlusStatic            2742446           437 ns/op          98 B/op           3 allocs/op
BenchmarkAce_GPlusParam               11626975           105 ns/op           0 B/op           0 allocs/op
BenchmarkAero_GPlusParam              16914322          71.6 ns/op           0 B/op           0 allocs/op
BenchmarkBear_GPlusParam               1405173           832 ns/op         480 B/op           5 allocs/op
BenchmarkBeego_GPlusParam              1000000          1075 ns/op         352 B/op           3 allocs/op
BenchmarkBone_GPlusParam               1000000          1557 ns/op         816 B/op           6 allocs/op
BenchmarkChi_GPlusParam                1347926           894 ns/op         432 B/op           3 allocs/op
BenchmarkDenco_GPlusParam              5513000           212 ns/op          64 B/op           1 allocs/op
BenchmarkEcho_GPlusParam              11884383           101 ns/op           0 B/op           0 allocs/op
BenchmarkGin_GPlusParam               12898952          93.1 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_GPlusParam         1000000          1194 ns/op         648 B/op           8 allocs/op
BenchmarkGoji_GPlusParam               1857229           645 ns/op         336 B/op           2 allocs/op
BenchmarkGojiv2_GPlusParam              520939          2322 ns/op        1328 B/op          11 allocs/op
BenchmarkGoJsonRest_GPlusParam         1000000          1536 ns/op         649 B/op          13 allocs/op
BenchmarkGoRestful_GPlusParam           205449          5800 ns/op        4192 B/op          14 allocs/op
BenchmarkGorillaMux_GPlusParam          395310          3188 ns/op        1280 B/op          10 allocs/op
BenchmarkGowwwRouter_GPlusParam        1851798           667 ns/op         432 B/op           3 allocs/op
BenchmarkHttpRouter_GPlusParam        18420789          65.2 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_GPlusParam        1878463           629 ns/op         352 B/op           3 allocs/op
BenchmarkKocha_GPlusParam              4495610           273 ns/op          56 B/op           3 allocs/op
BenchmarkLARS_GPlusParam              14615976          83.2 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_GPlusParam             584145          2549 ns/op        1072 B/op          10 allocs/op
BenchmarkMartini_GPlusParam             250501          4583 ns/op        1072 B/op          10 allocs/op
BenchmarkPat_GPlusParam                1000000          1645 ns/op         576 B/op          11 allocs/op
BenchmarkPossum_GPlusParam             1000000          1008 ns/op         496 B/op           5 allocs/op
BenchmarkR2router_GPlusParam           1708191           688 ns/op         432 B/op           5 allocs/op
BenchmarkRivet_GPlusParam              5795014           211 ns/op          48 B/op           1 allocs/op
BenchmarkTango_GPlusParam              1000000          1091 ns/op         264 B/op           8 allocs/op
BenchmarkTigerTonic_GPlusParam          760221          2489 ns/op         856 B/op          16 allocs/op
BenchmarkTraffic_GPlusParam             309774          4039 ns/op        1872 B/op          21 allocs/op
BenchmarkVulcan_GPlusParam             1935730           623 ns/op          98 B/op           3 allocs/op
BenchmarkAce_GPlus2Params              9158314           134 ns/op           0 B/op           0 allocs/op
BenchmarkAero_GPlus2Params            11300517           107 ns/op           0 B/op           0 allocs/op
BenchmarkBear_GPlus2Params             1239238           961 ns/op         496 B/op           5 allocs/op
BenchmarkBeego_GPlus2Params            1000000          1202 ns/op         352 B/op           3 allocs/op
BenchmarkBone_GPlus2Params              335576          3725 ns/op        1168 B/op          10 allocs/op
BenchmarkChi_GPlus2Params              1000000          1014 ns/op         432 B/op           3 allocs/op
BenchmarkDenco_GPlus2Params            4394598           280 ns/op          64 B/op           1 allocs/op
BenchmarkEcho_GPlus2Params             7851861           154 ns/op           0 B/op           0 allocs/op
BenchmarkGin_GPlus2Params              9958588           120 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_GPlus2Params       1000000          1433 ns/op         712 B/op           9 allocs/op
BenchmarkGoji_GPlus2Params             1325134           909 ns/op         336 B/op           2 allocs/op
BenchmarkGojiv2_GPlus2Params            405955          2870 ns/op        1408 B/op          14 allocs/op
BenchmarkGoJsonRest_GPlus2Params        977038          1987 ns/op         713 B/op          14 allocs/op
BenchmarkGoRestful_GPlus2Params         205018          6142 ns/op        4384 B/op          16 allocs/op
BenchmarkGorillaMux_GPlus2Params        205641          6015 ns/op        1296 B/op          10 allocs/op
BenchmarkGowwwRouter_GPlus2Params      1748542           684 ns/op         432 B/op           3 allocs/op
BenchmarkHttpRouter_GPlus2Params      14047102          87.7 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_GPlus2Params      1418673           828 ns/op         384 B/op           4 allocs/op
BenchmarkKocha_GPlus2Params            2334562           520 ns/op         128 B/op           5 allocs/op
BenchmarkLARS_GPlus2Params            11954094           101 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_GPlus2Params           491552          2890 ns/op        1072 B/op          10 allocs/op
BenchmarkMartini_GPlus2Params           120532          9545 ns/op        1200 B/op          13 allocs/op
BenchmarkPat_GPlus2Params               194739          6766 ns/op        2168 B/op          33 allocs/op
BenchmarkPossum_GPlus2Params           1201224          1009 ns/op         496 B/op           5 allocs/op
BenchmarkR2router_GPlus2Params         1575535           756 ns/op         432 B/op           5 allocs/op
BenchmarkRivet_GPlus2Params            3698930           325 ns/op          96 B/op           1 allocs/op
BenchmarkTango_GPlus2Params            1000000          1212 ns/op         344 B/op           8 allocs/op
BenchmarkTigerTonic_GPlus2Params        349350          3660 ns/op        1200 B/op          22 allocs/op
BenchmarkTraffic_GPlus2Params           169714          7862 ns/op        2248 B/op          28 allocs/op
BenchmarkVulcan_GPlus2Params           1222288           974 ns/op          98 B/op           3 allocs/op
BenchmarkAce_GPlusAll                   845606          1398 ns/op           0 B/op           0 allocs/op
BenchmarkAero_GPlusAll                 1000000          1009 ns/op           0 B/op           0 allocs/op
BenchmarkBear_GPlusAll                  103830         11386 ns/op        5488 B/op          61 allocs/op
BenchmarkBeego_GPlusAll                  82653         14784 ns/op        4576 B/op          39 allocs/op
BenchmarkBone_GPlusAll                   36601         33123 ns/op       11744 B/op         109 allocs/op
BenchmarkChi_GPlusAll                    95264         12831 ns/op        5616 B/op          39 allocs/op
BenchmarkDenco_GPlusAll                 567681          2950 ns/op         672 B/op          11 allocs/op
BenchmarkEcho_GPlusAll                  720366          1665 ns/op           0 B/op           0 allocs/op
BenchmarkGin_GPlusAll                  1000000          1185 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_GPlusAll             71575         16365 ns/op        8040 B/op         103 allocs/op
BenchmarkGoji_GPlusAll                  136352          9191 ns/op        3696 B/op          22 allocs/op
BenchmarkGojiv2_GPlusAll                 38006         31802 ns/op       17616 B/op         154 allocs/op
BenchmarkGoJsonRest_GPlusAll             57238         21561 ns/op        8117 B/op         170 allocs/op
BenchmarkGoRestful_GPlusAll              15147         79276 ns/op       55520 B/op         192 allocs/op
BenchmarkGorillaMux_GPlusAll             24446         48410 ns/op       16112 B/op         128 allocs/op
BenchmarkGowwwRouter_GPlusAll           150112          7770 ns/op        4752 B/op          33 allocs/op
BenchmarkHttpRouter_GPlusAll           1367820           878 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_GPlusAll           166628          8004 ns/op        4032 B/op          38 allocs/op
BenchmarkKocha_GPlusAll                 265694          4570 ns/op         976 B/op          43 allocs/op
BenchmarkLARS_GPlusAll                 1000000          1068 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_GPlusAll                54564         23305 ns/op        9568 B/op         104 allocs/op
BenchmarkMartini_GPlusAll                16274         73845 ns/op       14016 B/op         145 allocs/op
BenchmarkPat_GPlusAll                    27181         44478 ns/op       15264 B/op         271 allocs/op
BenchmarkPossum_GPlusAll                122587         10277 ns/op        5408 B/op          39 allocs/op
BenchmarkR2router_GPlusAll              130137          9297 ns/op        5040 B/op          63 allocs/op
BenchmarkRivet_GPlusAll                 532438          3323 ns/op         768 B/op          11 allocs/op
BenchmarkTango_GPlusAll                  86054         14531 ns/op        3656 B/op         104 allocs/op
BenchmarkTigerTonic_GPlusAll             33936         35356 ns/op       11600 B/op         242 allocs/op
BenchmarkTraffic_GPlusAll                17833         68181 ns/op       26248 B/op         341 allocs/op
BenchmarkVulcan_GPlusAll                120109          9861 ns/op        1274 B/op          39 allocs/op
```

## Parse.com

```sh
BenchmarkGin_ParseStatic              18877833          63.5 ns/op           0 B/op           0 allocs/op

BenchmarkAce_ParseStatic              19663731          60.8 ns/op           0 B/op           0 allocs/op
BenchmarkAero_ParseStatic             28967341          41.5 ns/op           0 B/op           0 allocs/op
BenchmarkBear_ParseStatic              3006984           402 ns/op         120 B/op           3 allocs/op
BenchmarkBeego_ParseStatic             1000000          1031 ns/op         352 B/op           3 allocs/op
BenchmarkBone_ParseStatic              1782482           675 ns/op         144 B/op           3 allocs/op
BenchmarkChi_ParseStatic               1453261           819 ns/op         432 B/op           3 allocs/op
BenchmarkDenco_ParseStatic            45023595          26.5 ns/op           0 B/op           0 allocs/op
BenchmarkEcho_ParseStatic             17330470          69.3 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_ParseStatic        1644006           731 ns/op         296 B/op           5 allocs/op
BenchmarkGoji_ParseStatic              7026930           170 ns/op           0 B/op           0 allocs/op
BenchmarkGojiv2_ParseStatic             517618          2037 ns/op        1312 B/op          10 allocs/op
BenchmarkGoJsonRest_ParseStatic        1227080           975 ns/op         329 B/op          11 allocs/op
BenchmarkGoRestful_ParseStatic          192458          6659 ns/op        4256 B/op          13 allocs/op
BenchmarkGorillaMux_ParseStatic         744062          2109 ns/op         976 B/op           9 allocs/op
BenchmarkGowwwRouter_ParseStatic      37781062          31.8 ns/op           0 B/op           0 allocs/op
BenchmarkHttpRouter_ParseStatic       45311223          26.5 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_ParseStatic      21383475          56.1 ns/op           0 B/op           0 allocs/op
BenchmarkKocha_ParseStatic            29953290          40.1 ns/op           0 B/op           0 allocs/op
BenchmarkLARS_ParseStatic             20036196          62.7 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_ParseStatic           1000000          1740 ns/op         736 B/op           8 allocs/op
BenchmarkMartini_ParseStatic            404156          3801 ns/op         768 B/op           9 allocs/op
BenchmarkPat_ParseStatic               1547180           772 ns/op         240 B/op           5 allocs/op
BenchmarkPossum_ParseStatic            1608991           757 ns/op         416 B/op           3 allocs/op
BenchmarkR2router_ParseStatic          3177936           385 ns/op         144 B/op           4 allocs/op
BenchmarkRivet_ParseStatic            17783205          67.4 ns/op           0 B/op           0 allocs/op
BenchmarkTango_ParseStatic             1210777           990 ns/op         248 B/op           8 allocs/op
BenchmarkTigerTonic_ParseStatic        5316440           231 ns/op          48 B/op           1 allocs/op
BenchmarkTraffic_ParseStatic            496050          2539 ns/op        1256 B/op          19 allocs/op
BenchmarkVulcan_ParseStatic            2462798           488 ns/op          98 B/op           3 allocs/op
BenchmarkAce_ParseParam               13393669          89.6 ns/op           0 B/op           0 allocs/op
BenchmarkAero_ParseParam              19836619          60.4 ns/op           0 B/op           0 allocs/op
BenchmarkBear_ParseParam               1405954           864 ns/op         467 B/op           5 allocs/op
BenchmarkBeego_ParseParam              1000000          1065 ns/op         352 B/op           3 allocs/op
BenchmarkBone_ParseParam               1000000          1698 ns/op         896 B/op           7 allocs/op
BenchmarkChi_ParseParam                1356037           873 ns/op         432 B/op           3 allocs/op
BenchmarkDenco_ParseParam              6241392           204 ns/op          64 B/op           1 allocs/op
BenchmarkEcho_ParseParam              14088100          85.1 ns/op           0 B/op           0 allocs/op
BenchmarkGin_ParseParam               17426064          68.9 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_ParseParam         1000000          1254 ns/op         664 B/op           8 allocs/op
BenchmarkGoji_ParseParam               1682574           713 ns/op         336 B/op           2 allocs/op
BenchmarkGojiv2_ParseParam              502224          2333 ns/op        1360 B/op          12 allocs/op
BenchmarkGoJsonRest_ParseParam         1000000          1401 ns/op         649 B/op          13 allocs/op
BenchmarkGoRestful_ParseParam           182623          7097 ns/op        4576 B/op          14 allocs/op
BenchmarkGorillaMux_ParseParam          482332          2477 ns/op        1280 B/op          10 allocs/op
BenchmarkGowwwRouter_ParseParam        1834873           657 ns/op         432 B/op           3 allocs/op
BenchmarkHttpRouter_ParseParam        23593393          51.0 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_ParseParam        2100160           574 ns/op         352 B/op           3 allocs/op
BenchmarkKocha_ParseParam              4837220           252 ns/op          56 B/op           3 allocs/op
BenchmarkLARS_ParseParam              18411192          66.2 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_ParseParam             571870          2398 ns/op        1072 B/op          10 allocs/op
BenchmarkMartini_ParseParam             286262          4268 ns/op        1072 B/op          10 allocs/op
BenchmarkPat_ParseParam                 692906          2157 ns/op         992 B/op          15 allocs/op
BenchmarkPossum_ParseParam             1000000          1011 ns/op         496 B/op           5 allocs/op
BenchmarkR2router_ParseParam           1722735           697 ns/op         432 B/op           5 allocs/op
BenchmarkRivet_ParseParam              6058054           203 ns/op          48 B/op           1 allocs/op
BenchmarkTango_ParseParam              1000000          1061 ns/op         280 B/op           8 allocs/op
BenchmarkTigerTonic_ParseParam          890275          2277 ns/op         784 B/op          15 allocs/op
BenchmarkTraffic_ParseParam             351322          3543 ns/op        1896 B/op          21 allocs/op
BenchmarkVulcan_ParseParam             2076544           572 ns/op          98 B/op           3 allocs/op
BenchmarkAce_Parse2Params             11718074           101 ns/op           0 B/op           0 allocs/op
BenchmarkAero_Parse2Params            16264988          73.4 ns/op           0 B/op           0 allocs/op
BenchmarkBear_Parse2Params             1238322           973 ns/op         496 B/op           5 allocs/op
BenchmarkBeego_Parse2Params            1000000          1120 ns/op         352 B/op           3 allocs/op
BenchmarkBone_Parse2Params             1000000          1632 ns/op         848 B/op           6 allocs/op
BenchmarkChi_Parse2Params              1239477           955 ns/op         432 B/op           3 allocs/op
BenchmarkDenco_Parse2Params            4944133           245 ns/op          64 B/op           1 allocs/op
BenchmarkEcho_Parse2Params            10518286           114 ns/op           0 B/op           0 allocs/op
BenchmarkGin_Parse2Params             14505195          82.7 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_Parse2Params       1000000          1437 ns/op         712 B/op           9 allocs/op
BenchmarkGoji_Parse2Params             1689883           707 ns/op         336 B/op           2 allocs/op
BenchmarkGojiv2_Parse2Params            502334          2308 ns/op        1344 B/op          11 allocs/op
BenchmarkGoJsonRest_Parse2Params       1000000          1771 ns/op         713 B/op          14 allocs/op
BenchmarkGoRestful_Parse2Params         159092          7583 ns/op        4928 B/op          14 allocs/op
BenchmarkGorillaMux_Parse2Params        417548          2980 ns/op        1296 B/op          10 allocs/op
BenchmarkGowwwRouter_Parse2Params      1751737           686 ns/op         432 B/op           3 allocs/op
BenchmarkHttpRouter_Parse2Params      18089204          66.3 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_Parse2Params      1556986           777 ns/op         384 B/op           4 allocs/op
BenchmarkKocha_Parse2Params            2493082           485 ns/op         128 B/op           5 allocs/op
BenchmarkLARS_Parse2Params            15350108          78.5 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_Parse2Params           530974          2605 ns/op        1072 B/op          10 allocs/op
BenchmarkMartini_Parse2Params           247069          4673 ns/op        1152 B/op          11 allocs/op
BenchmarkPat_Parse2Params               816295          2126 ns/op         752 B/op          16 allocs/op
BenchmarkPossum_Parse2Params           1000000          1002 ns/op         496 B/op           5 allocs/op
BenchmarkR2router_Parse2Params         1569771           733 ns/op         432 B/op           5 allocs/op
BenchmarkRivet_Parse2Params            4080546           295 ns/op          96 B/op           1 allocs/op
BenchmarkTango_Parse2Params            1000000          1121 ns/op         312 B/op           8 allocs/op
BenchmarkTigerTonic_Parse2Params        399556          3470 ns/op        1168 B/op          22 allocs/op
BenchmarkTraffic_Parse2Params           314194          4159 ns/op        1944 B/op          22 allocs/op
BenchmarkVulcan_Parse2Params           1827559           664 ns/op          98 B/op           3 allocs/op
BenchmarkAce_ParseAll                   478395          2503 ns/op           0 B/op           0 allocs/op
BenchmarkAero_ParseAll                  715392          1658 ns/op           0 B/op           0 allocs/op
BenchmarkBear_ParseAll                   59191         20124 ns/op        8928 B/op         110 allocs/op
BenchmarkBeego_ParseAll                  45507         27266 ns/op        9152 B/op          78 allocs/op
BenchmarkBone_ParseAll                   29328         41459 ns/op       16208 B/op         147 allocs/op
BenchmarkChi_ParseAll                    48531         25053 ns/op       11232 B/op          78 allocs/op
BenchmarkDenco_ParseAll                 325532          4284 ns/op         928 B/op          16 allocs/op
BenchmarkEcho_ParseAll                  433771          2759 ns/op           0 B/op           0 allocs/op
BenchmarkGin_ParseAll                   576316          2082 ns/op           0 B/op           0 allocs/op
BenchmarkGocraftWeb_ParseAll             41500         29692 ns/op       13728 B/op         181 allocs/op
BenchmarkGoji_ParseAll                   80833         15563 ns/op        5376 B/op          32 allocs/op
BenchmarkGojiv2_ParseAll                 19836         60335 ns/op       34448 B/op         277 allocs/op
BenchmarkGoJsonRest_ParseAll             32210         38027 ns/op       13866 B/op         321 allocs/op
BenchmarkGoRestful_ParseAll               6644        190842 ns/op      117600 B/op         354 allocs/op
BenchmarkGorillaMux_ParseAll             12634         95894 ns/op       30288 B/op         250 allocs/op
BenchmarkGowwwRouter_ParseAll            98152         12159 ns/op        6912 B/op          48 allocs/op
BenchmarkHttpRouter_ParseAll            933208          1273 ns/op           0 B/op           0 allocs/op
BenchmarkHttpTreeMux_ParseAll           107191         11554 ns/op        5728 B/op          51 allocs/op
BenchmarkKocha_ParseAll                 184862          6225 ns/op        1112 B/op          54 allocs/op
BenchmarkLARS_ParseAll                  644546          1858 ns/op           0 B/op           0 allocs/op
BenchmarkMacaron_ParseAll                26145         46484 ns/op       19136 B/op         208 allocs/op
BenchmarkMartini_ParseAll                10000        121838 ns/op       25072 B/op         253 allocs/op
BenchmarkPat_ParseAll                    25417         47196 ns/op       15216 B/op         308 allocs/op
BenchmarkPossum_ParseAll                 58550         20735 ns/op       10816 B/op          78 allocs/op
BenchmarkR2router_ParseAll               72732         16584 ns/op        8352 B/op         120 allocs/op
BenchmarkRivet_ParseAll                 281365          4968 ns/op         912 B/op          16 allocs/op
BenchmarkTango_ParseAll                  42831         28668 ns/op        7168 B/op         208 allocs/op
BenchmarkTigerTonic_ParseAll             23774         49972 ns/op       16048 B/op         332 allocs/op
BenchmarkTraffic_ParseAll                10000        104679 ns/op       45520 B/op         605 allocs/op
BenchmarkVulcan_ParseAll                 64810         18108 ns/op        2548 B/op          78 allocs/op
```

## CONTRIBUTING.md

## Contributing

- With issues:
  - Use the search tool before opening a new issue.
  - Please provide source code and commit sha if you found a bug.
  - Review existing issues and provide feedback or react to them.

- With pull requests:
  - Open your pull request against `master`
  - Your pull request should have no more than two commits, if not you should squash them.
  - It should pass all tests in the available continuous integration systems such as GitHub Actions.
  - You should add/modify tests to cover your proposed code changes.
  - If your pull request contains a new feature, please document it on the README.

