@echo off
chcp 65001 >nul
cd /d "%~dp0"
title VaiJunto - Motorista
bin\motorista.exe
echo.
echo Cliente motorista encerrado.
pause
