# Export all .drawio files in this directory to PNG in the ./image folder
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$outputDir = Join-Path $scriptDir "image"

# draw.io executable path
$drawio = "C:\Program Files\draw.io\draw.io.exe"

# Create output directory if it doesn't exist
if (-not (Test-Path $outputDir)) {
    New-Item -ItemType Directory -Path $outputDir | Out-Null
}

# Get all .drawio files
$files = Get-ChildItem -Path $scriptDir -Filter "*.drawio"

if ($files.Count -eq 0) {
    Write-Host "No .drawio files found in $scriptDir"
    exit 0
}

Write-Host "Found $($files.Count) .drawio files to convert..."
Write-Host ""

$success = 0
$failed = 0

foreach ($file in $files) {
    $outputFile = Join-Path $outputDir ($file.BaseName + ".png")
    Write-Host "Converting $($file.Name) -> image/$($file.BaseName).png ..." -NoNewline

    # Use Start-Process -Wait to ensure each conversion finishes before the next
    $proc = Start-Process -FilePath $drawio -ArgumentList "-x", "-f", "png", "-s", "2", "-t", "-o", "`"$outputFile`"", "`"$($file.FullName)`"" -Wait -PassThru -NoNewWindow -RedirectStandardError "NUL" 2>$null

    if (Test-Path $outputFile) {
        $size = (Get-Item $outputFile).Length
        Write-Host " OK ($([math]::Round($size/1024))KB)" -ForegroundColor Green
        $success++
    } else {
        Write-Host " FAILED" -ForegroundColor Red
        $failed++
    }

    # Small delay to avoid Electron conflicts
    Start-Sleep -Milliseconds 500
}

Write-Host ""
Write-Host "Done! $success succeeded, $failed failed out of $($files.Count) files." -ForegroundColor Cyan
