include $(GOROOT)/src/Make.$(GOARCH)

TARG=gogallery

GOFILES=\
	http.go \
	sql.go \
	html.go \
	main.go

include $(GOROOT)/src/Make.cmd
