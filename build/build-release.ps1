param (
    [string]$oses,
    [string]$arch,
    [string]$ldflags,
    [string]$filename,
    [string]$pkg,
    [string]$bindir
)
$env:CGO_ENABLED=0
$env:GO111MODULE='on'
$env:GOARCH=$arch
foreach ($filename_os in $oses -split " ") {
    $env:GOOS=${filename_os}
    go build -ldflags $ldflags -o ${bindir}/${filename}-${filename_os}-${arch} ${pkg}/cmd/...;
    if (${filename_os} -eq 'windows') {
            Move-Item -Force ${bindir}/${filename}-${filename_os}-${arch} ${bindir}/${filename}-${filename_os}-${arch}.exe
    } 
}
