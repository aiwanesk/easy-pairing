@echo off
rem Compile et lance pairing_easy depuis les sources (double-clic).
rem Modes : start-windows.bat [run|dev|build|test] [options de pairing_easy]
chcp 65001 >nul
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0build\dev-windows.ps1" %*
if errorlevel 1 (
  echo.
  echo Une erreur est survenue. Appuyez sur une touche pour fermer.
  pause >nul
)
