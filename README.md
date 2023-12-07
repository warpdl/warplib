# WarpLib

The core library that powers up WarpDL.

## Installation
You can download the library with the help of standard `go get` command.

```bash
go get github.com/warpdl/warplib
```

## Usage
```go
package main

import (
	"log"
	"net/http"
	"github.com/warpdl/warplib"
)

func main() {
        fileToDownload := "https://download.com/file.zip"
	    d, err := warplib.NewDownloader(
            &http.Client{},
    		fileToDownload,
    		&warplib.DownloaderOpts{
    			ForceParts: true,
    			Handlers: &warplib.Handlers{
    				ErrorHandler: func(hash string, err error) {
    					log.Println("Failed to continue downloading:", rectifyError(err))
    					os.Exit(0)
    				},
    				DownloadCompleteHandler: func(hash string, tread int64) {
    					log.Println("Download Complete!")
    				},
    			},
    			MaxConnections:    24,
    			MaxSegments:       200,
    			FileName:          "fileName.zip",
    			DownloadDirectory: ".",
    		},
	)
    if err != nil {
        log.Println("failed to create downloader:", err)
        return
    }
    err = d.Start()
    if err != nil {
        log.Println("failed to start download:", err)
        return
    }
}
```

## **Contributing**

Pull requests and stars are always welcome. For bugs and feature requests, [please create an issue](../../issues/new).

## **License**

Copyright © 2023, [WarpDL](https://github.com/warpdl).
Released under the [MIT License](LICENSE).
