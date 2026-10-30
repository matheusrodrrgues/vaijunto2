@echo off
chcp 65001 >nul
cd /d "%~dp0"
title VaiJunto - Passageiro
bin\cliente.exe
echo.
echo Cliente passageiro encerrado.
pause
