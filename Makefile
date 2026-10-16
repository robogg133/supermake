# using base-musl
# include git.servidordomal.lol/robogg133/go
export CGO_ENABLED=0

supermake:
	go build -o supermake -trimpath -ldflags="-s -w" .

clean:
	-rm supermake
