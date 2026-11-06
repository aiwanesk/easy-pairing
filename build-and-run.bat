@echo off
rem Compile et lance pairing_easy depuis les sources.
rem Decompresse source.zip au premier lancement, puis appelle source\start-windows.bat.
chcp 65001 >nul
cd /d "%~dp0"
if not exist "source\start-windows.bat" (
  if not exist "source.zip" (
    echo source.zip est introuvable a cote de ce script.
    pause
    exit /b 1
  )
  echo Decompression de source.zip...
  powershell -NoProfile -ExecutionPolicy Bypass -Command "Expand-Archive -Force -LiteralPath 'source.zip' -DestinationPath '.'"
  if errorlevel 1 (
    echo La decompression a echoue.
    pause
    exit /b 1
  )
)
call "source\start-windows.bat" %*
