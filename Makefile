all: 
	@echo "Started normal build.."
	go build 
	@echo "Completed normal build."

small: 
	@echo "Started binary size optimized build. (-stripping debug info, disabled CGO)"
	CGO_ENALBED=0 go build -ldflags="-s -w" -trimpath -o gozart main.go
	@echo "Done. (you could see around ~35% reduction in binary size)"

