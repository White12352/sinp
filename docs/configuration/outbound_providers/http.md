### Structure

```json
{
  "type": "file",
  "tag": "file",
  "path": "./file.json",
  "healthcheck_url": "https://www.gstatic.com/generate_204",

  "download_url": "http://www.baidu.com",
  "download_ua": "sing-box",
  "download_detour": ""
}
```

### Fields

#### download_url

==Required==

The download URL of the outbound-provider.

#### download_ua

The `User-Agent` used for downloading outbound-provider.

Default is `sing-box`.

#### download_detour

The tag of the outbound to download the database.

Default outbound will be used if empty.
