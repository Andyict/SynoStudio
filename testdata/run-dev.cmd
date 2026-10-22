@echo off
rem Local dev run: fake shares in testdata\shares, data in testdata\data, console without DSM auth.
cd /d "%~dp0"
set STUDIO_DEV=1
set STUDIO_SHARES_ROOT=%~dp0shares
"%~dp0studioalbum.exe" serve -data "%~dp0data" -port 8686
