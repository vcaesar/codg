package main

import _ "embed"

//go:embed build/appicon.png
var appIcon []byte

//go:embed build/tray-template.png
var trayTemplateIcon []byte
