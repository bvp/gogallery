include $(GOROOT)/src/Make.$(GOARCH)

TARG=gogallery

$(TARG): gogallery.go
	$(GC) gogallery.go
	$(LD) -o $(TARG) gogallery.$O

all: $(TARG)
	
clean:
	rm *.$O $(TARG)
