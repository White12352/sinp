### 结构

```json
{
  "type": "http",
  "tag": "http",
  "path": "./http.json",
  "healthcheck_url": "https://www.gstatic.com/generate_204",

  "download_url": "http://www.baidu.com",
  "download_ua": "sing-box",
  "download_detour": ""
}
```

#### download_url

==必填==

指定出站提供者的下载链接。

#### download_ua

指定出站提供者的下载时的 `User-Agent`。

默认为 `sing-box`。

#### download_detour

用于下载出站提供者的出站的标签。

如果为空，将使用默认出站。
