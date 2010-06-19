include $(GOROOT)/src/Make.$(GOARCH)

TARG=gogallery

GOFILES=\
	http.go \
	sql.go \
	main.go

include $(GOROOT)/src/Make.cmd
