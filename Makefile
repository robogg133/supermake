#include git.servidordomal.lol/robogg133/go-supermake 1.27.0
# config out set out/*
export CGO_ENABLED=0

out/bin/supermake:
	go build -o out/bin/supermake -trimpath -ldflags="-s -w" .

clean:
	-rm -r out
