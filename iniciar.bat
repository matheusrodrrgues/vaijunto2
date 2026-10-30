@echo off
chcp 65001 >nul
cd /d "%~dp0"
title VaiJunto - Servidor
bin\servidor.exe
echo.
echo Servidor encerrado.
pause
