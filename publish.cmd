@echo off
echo ========================================
echo   DelayForge Publish Script
echo ========================================
echo.

echo [1] Building self-contained (no .NET required, ~60MB)...
dotnet publish src\DelayForge\DelayForge.csproj -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true -p:EnableCompressionInSingleFile=true -p:IncludeNativeLibrariesForSelfExtract=true -p:IncludeAllContentForSelfExtract=true -o publish\self-contained
if %errorlevel% neq 0 goto :error

echo.
echo [2] Building framework-dependent (~1MB, requires .NET 9 runtime)...
dotnet publish src\DelayForge\DelayForge.csproj -c Release -r win-x64 --self-contained false -p:PublishSingleFile=true -o publish\framework-dependent
if %errorlevel% neq 0 goto :error

echo.
echo ========================================
echo   Done! Output:
echo ========================================
echo   self-contained:      publish\self-contained\DelayForge.exe
echo   framework-dependent: publish\framework-dependent\DelayForge.exe
echo.
dir publish\self-contained\DelayForge.exe | findstr "DelayForge"
dir publish\framework-dependent\DelayForge.exe | findstr "DelayForge"
echo ========================================
goto :end

:error
echo.
echo BUILD FAILED!
exit /b 1

:end
