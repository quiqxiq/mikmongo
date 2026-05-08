# Export all .puml files in this directory to PNG in the ./image folder
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$outputDir = Join-Path $scriptDir "image"
$plantumlJar = Join-Path (Split-Path -Parent $scriptDir) "plantuml.jar"

# Verify plantuml.jar exists
if (-not (Test-Path $plantumlJar)) {
    Write-Host "ERROR: plantuml.jar not found at $plantumlJar" -ForegroundColor Red
    Write-Host "Download it from https://plantuml.com/download"
    exit 1
}

# Create output directory if it doesn't exist
if (-not (Test-Path $outputDir)) {
    New-Item -ItemType Directory -Path $outputDir | Out-Null
}

# Get all .puml files
$files = Get-ChildItem -Path $scriptDir -Filter "*.puml"

if ($files.Count -eq 0) {
    Write-Host "No .puml files found in $scriptDir"
    exit 0
}

Write-Host "Found $($files.Count) .puml files to convert..."
Write-Host "Using: $plantumlJar"
Write-Host ""

$success = 0
$failed = 0

foreach ($file in $files) {
    $outputFile = Join-Path $outputDir ($file.BaseName + ".png")
    Write-Host "Converting $($file.Name) -> image/$($file.BaseName).png ..." -NoNewline

    # PlantUML export: -tpng for PNG, -o for output directory, -scale 2 for high-res
    $proc = Start-Process -FilePath "java" `
        -ArgumentList "-jar", "`"$plantumlJar`"", "-tpng", "-scale", "2", "-o", "`"$outputDir`"", "`"$($file.FullName)`"" `
        -Wait -PassThru -NoNewWindow -RedirectStandardError "NUL" 2>$null

    if (Test-Path $outputFile) {
        $size = (Get-Item $outputFile).Length
        Write-Host " OK ($([math]::Round($size/1024))KB)" -ForegroundColor Green
        $success++
    } else {
        Write-Host " FAILED" -ForegroundColor Red
        $failed++
    }
}

Write-Host ""
Write-Host "Done! $success succeeded, $failed failed out of $($files.Count) files." -ForegroundColor Cyan
