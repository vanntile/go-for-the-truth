module github.com/vanntile/go-for-the-truth

go 1.23.1

require (
	github.com/a-h/templ v0.2.778
	github.com/kennygrant/sanitize v1.2.4
	github.com/labstack/echo/v4 v4.12.0
	github.com/labstack/gommon v0.4.2
	github.com/mattn/go-sqlite3 v1.14.24
	github.com/rs/zerolog v1.29.0
	github.com/ziflex/lecho/v3 v3.7.0
	golang.org/x/crypto v0.26.0
	golang.org/x/text v0.17.0
	golang.org/x/time v0.5.0
)

require (
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sys v0.23.0 // indirect
)

replace github.com/mattn/go-sqlite3 => github.com/jgiannuzzi/go-sqlite3 v1.14.17-0.20230223050003-85a15a7254f2
