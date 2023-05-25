# WarpLib

The core library that powers up WARP, an ultra fast download manager.

## Getting Started

### **Installing the library**
Here, we will discuss to install warplib and use it projects.

**Prerequisites**:

- Have at least `read` access to this repository (which you already have since you're reading this document) on the account authorized on `git` present in your device. 

- Have a Go installation with version greater than or equal to `v1.19`.

- Include warplib in `GOPRIVATE` environment variable of your Go installation.
<br>

**Setting up `GOPRIVATE`**

**warplib** is a private go module hence you will need to add it in `GOPRIVATE` env variable so that it doesn't get cached on google's public mirror proxy and to avoid go verifying its checksum in [GOSUMDB](sum.golang.org).

You need to firstly verify that whether your    `GOPRIVATE` env var is contains anything or not, for that you need to run the following command:
```sh
$ go env GOPRIVATE
``` 

If the output is empty then you don't have any existing private go module configured and vice versa. We will discuss both the cases, you may follow whichever suits your configuration: 

- `GOPRIVATE` is empty
    
    You don't need to worry much if your `GOPRIVATE` is already empty and just run the following command in your terminal:
    ```sh
    $ go env -w GOPRIVATE=github.com/warpdl/warplib
    ```
- `GOPRIVATE` is not empty
    
    If the output of `GOPRIVATE` is not empty, this means that you already have private go module present in your configuration and hence you would need to append warpdl's module to the existing output, separated by a comma (`,`).

    Let us suppose that the output for `GOPRIVATE` was `github.com/golang/package` then you will have to run the following command in your terminal:
    ```sh
    $ go env -w GOPRIVATE=github.com/golang/package,github.com/warpdl/warplib
    ```

Congratulations! You have successfully configured the private go module for warplib.

<br>

**Including `warplib` in your project**

You can include warplib in your project by using standard `go get` command after succesfully configuring things stated above. 
```sh
$ go get github.com/warpdl/warplib
```
<br>

### **Contributing**

Pull requests and stars are always welcome. For bugs and feature requests, [please create an issue](../../issues/new).

### **Author**

**Veer Pratap Singh**

* [github/anonyindian](https://github.com/anonyindian)
* [telegram/CaptainPicard](https://t.me/CaptainPicard)

### **License**

Copyright © 2023, [WarpDL](https://github.com/jonschlinkert).
Released under the [AGPL-V3 License](LICENSE).

