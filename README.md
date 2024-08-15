# v2sub

用于将不标准的梯子订阅链接转换为标准的链接。

在使用 v2rayU 的时候，遇到了某些机场的订阅链接返回数据无法识别的问题，后来发现是因为链接中 # 锚点部分后的内容（其实就是节点名称）没有 urlencode，所以写了这个小程序来修正这个错误。

## 原理

```
     节点数据            修正后的数据 
机场 ----------> v2sub ------------> 客户端
                  ^ 
                  ^ 监听本地端口
                  ^ 把机场返回的链接作了修正之后返回
```

## 使用方法

参数：

- `-subUrl` 机场提供的订阅 url
- `-proxyUrl` 可能需要通过代理访问机场提供的订阅 url，可以传入形如 `sock5://127.0.0.1:1080` 的代理地址，http 也支持。
- `-listen` 可以指定 v2sub 的监听地址，默认是 `127.0.0.1:18888`

---------

# v2sub

A tool for converting non-standard proxy subscription links into standard links.

When using v2rayU, I encountered issues with subscription links from certain proxy services returning data that couldn't be recognized. Later, I discovered that the problem was due to the content after the `#` anchor part of the link (which is essentially the node name) not being URL-encoded. So, I wrote this small program to fix this error.

## Principle

```
     Node Data           Corrected Data 
Service Provider -----> v2sub ------------> Client
                         ^ 
                         ^ Listens on a local port
                         ^ Returns the corrected link after fixing the one provided by the service
```

## Usage

Parameters:

- `-subUrl`: The subscription URL provided by the service provider.
- `-proxyUrl`: You might need to access the subscription URL through a proxy, which can be passed as a proxy address like `sock5://127.0.0.1:1080`. HTTP is also supported.
- `-listen`: You can specify the listening address for v2sub, with the default being `127.0.0.1:18888`.