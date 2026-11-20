# 打包脚本：按当前版本号构建 -> 生成 LeishenPause-<版本>.exe 和 version.txt -> 版本号 +0.1
$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
$vf = Join-Path $root 'version.go'
Set-Location $root
$utf8 = New-Object System.Text.UTF8Encoding $false

$src = [IO.File]::ReadAllText($vf, [Text.Encoding]::UTF8)
$ver = [regex]::Match($src, 'appVersion = "([\d.]+)"').Groups[1].Value
if (-not $ver) { throw 'version.go 里找不到 appVersion' }

Get-Process LeishenPause, '雷神自动暂停' -ErrorAction SilentlyContinue | Stop-Process -Force
$env:GOTOOLCHAIN = 'local'

go build -trimpath -ldflags "-s -w -H windowsgui" -o LeishenPause.exe .
if ($LASTEXITCODE -ne 0) { throw 'go build 失败' }

Copy-Item (Join-Path $root 'LeishenPause.exe') (Join-Path $root "LeishenPause-$ver.exe") -Force

$parts = $ver -split '\.'
$next = "$($parts[0]).$([int]$parts[1] + 1)"
[IO.File]::WriteAllText($vf, ($src -replace 'appVersion = "[\d.]+"', "appVersion = `"$next`""), $utf8)

"已打包 LeishenPause-$ver.exe；version.go 下一版本号：$next"
