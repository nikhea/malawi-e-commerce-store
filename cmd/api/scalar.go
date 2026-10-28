package main

import (
	"github.com/gin-gonic/gin"
)

// scalarPage serves the Scalar API reference (dark) reading our
// swagger.json. Zero Go deps — one CDN script. Tradeoff, stated: /docs
// needs internet for the CDN; /swagger (embedded swagger-ui) works
// offline. If offline docs matter, vendor the Scalar bundle locally.
const scalarPage = `<!doctype html>
<html>
<head>
<title>Malawi Store API — Docs</title>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<style>html,body{margin:0;background:#0f0f0f}</style>
</head>
<body>
<div id="app"></div>
<script type="module">
import { createApiReference } from 'https://cdn.jsdelivr.net/npm/@scalar/api-reference/esm.js'
createApiReference('#app', {
  url: '/swagger/doc.json',
  theme: 'deepSpace',
  darkMode: true,
})
</script>
</body>
</html>`

// registerScalarDocs mounts the dark API reference at /docs.
func registerScalarDocs(r *gin.Engine) {
	r.GET("/docs", func(c *gin.Context) {
		c.Data(200, "text/html; charset=utf-8", []byte(scalarPage))
	})
}
