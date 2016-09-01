package utils

import (
	color "github.com/fatih/color"
)

// 警告信息采用红色显示
var Red = color.New(color.FgRed).SprintFunc()

// 新增服务等采用绿色显示
var Green = color.New(color.FgGreen).SprintFunc()

var Magenta = color.New(color.FgMagenta).SprintFunc()
var Cyan = color.New(color.FgCyan).SprintFunc()

var Blue = color.New(color.FgBlue).SprintFunc()
