@echo off
cd ..
set CGO_ENABLED = 1
set GOOS = windows
set GOEXPERIMENT=arenas

if not exist go.mod (
	echo Missing dependencies, please run get.cmd
	echo.
	pause
	exit
)
if not exist bin (
	MKDIR bin
) 

echo Building Ikemen GO...
echo. 

go1.20rc2 build -ldflags "-s -w" -trimpath -v  -o ./bin/Ikemen_GO.exe ./src

pause