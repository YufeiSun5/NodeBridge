param()
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$root = Split-Path -Parent $PSScriptRoot
$dir = Join-Path $root 'build/installer-art'
New-Item -ItemType Directory -Path $dir -Force | Out-Null
$logo = [Drawing.Image]::FromFile((Join-Path $root 'build/appicon.png'))
try {
    foreach ($kind in @('welcome','header')) {
        $width,$height = if ($kind -eq 'welcome') { 164,314 } else { 150,57 }
        $bitmap = New-Object Drawing.Bitmap($width,$height)
        $graphics = [Drawing.Graphics]::FromImage($bitmap)
        $ink = New-Object Drawing.SolidBrush([Drawing.Color]::FromArgb(235,238,240))
        $muted = New-Object Drawing.SolidBrush([Drawing.Color]::FromArgb(163,173,177))
        $accent = New-Object Drawing.SolidBrush([Drawing.Color]::FromArgb(27,188,153))
        $font = New-Object Drawing.Font('Segoe UI',10,[Drawing.FontStyle]::Bold)
        $small = New-Object Drawing.Font('Segoe UI',8,[Drawing.FontStyle]::Regular)
        try {
            $graphics.Clear([Drawing.Color]::FromArgb(27,31,34))
            $graphics.TextRenderingHint = [Drawing.Text.TextRenderingHint]::AntiAliasGridFit
            $graphics.InterpolationMode = [Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
            if ($kind -eq 'welcome') {
                $graphics.DrawImage($logo,20,30,72,72)
                $graphics.DrawString('NODEBRIDGE',$font,$ink,18,122)
                $graphics.FillRectangle($accent,20,153,30,3)
                $graphics.DrawString("MySQL data sync`nEdge + Central",$small,$muted,18,177)
                $graphics.DrawString('WINDOWS',$small,$muted,18,280)
            } else {
                $graphics.DrawImage($logo,8,8,40,40)
                $graphics.DrawString('NodeBridge',$font,$ink,50,18)
            }
            $bitmap.Save((Join-Path $dir "$kind.bmp"),[Drawing.Imaging.ImageFormat]::Bmp)
        } finally {
            $small.Dispose(); $font.Dispose(); $accent.Dispose(); $muted.Dispose(); $ink.Dispose(); $graphics.Dispose(); $bitmap.Dispose()
        }
    }
} finally { $logo.Dispose() }
