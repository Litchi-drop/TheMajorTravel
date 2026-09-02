// Package web 过渡期静态托管（报告 §4.2 Step 2）：单个 index.html 内嵌进二进制。
// Step 3 换 Vue 构建产物 + 嵌入 dist/。
package web

import (
	"embed"
	"io/fs"
)

//go:embed index.html
var dist embed.FS

func Dist() fs.FS { return dist }
