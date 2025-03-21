module github.com/vanntile/go-for-the-truth

go 1.24.1

require (
	github.com/a-h/templ v0.3.833
	github.com/kennygrant/sanitize v1.2.4
	github.com/labstack/echo/v4 v4.13.3
	github.com/labstack/gommon v0.4.2
	github.com/mattn/go-sqlite3 v1.14.24
	github.com/rs/zerolog v1.33.0
	github.com/ziflex/lecho/v3 v3.7.0
	golang.org/x/text v0.23.0
	golang.org/x/time v0.11.0
)

require (
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/net v0.37.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
)

replace github.com/mattn/go-sqlite3 => github.com/jgiannuzzi/go-sqlite3 v1.14.17-0.20230223050003-85a15a7254f2
