@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"
if not exist bin mkdir bin

where go >nul 2>nul
if errorlevel 1 (
    echo O Go não foi encontrado no PATH.
    echo Instale o Go e tente novamente.
    pause
    exit /b 1
)

echo Compilando servidor...
go build -o bin\servidor.exe .\servidor
if errorlevel 1 goto erro
echo Compilando motorista...
go build -o bin\motorista.exe .\motorista
if errorlevel 1 goto erro
echo Compilando passageiro...
go build -o bin\cliente.exe .\cliente
if errorlevel 1 goto erro
echo.
echo Compilação concluída.
exit /b 0

:erro
echo.
echo A compilação falhou.
pause
exit /b 1
