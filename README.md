# Douyu Crawler Demo

斗鱼关注人数爬虫 Demo，具体可以参考这篇博客 [斗鱼关注人数爬取 —— 字体反爬的攻与防](https://cjting.me/2020/07/01/douyu-crawler-and-font-anti-crawling/)。

注意：**爬虫程序有很高的时效性，很快就会过时无法使用。Demo 最后测试时间为 2020-07-02 日**。

## 安装

```bash
$ go get -v github.com/cj1128/douyu-crawler-demo
```

## 爬取主播关注人数

通过命令行参数传递 roomID。

```bash
$ douyu-crawler-demo 2947432
2020/07/02 13:21:37 room ids: [2947432]
[2947432] 13:21:37 start to crawl followed count
[2947432] 13:21:37 followed_count fetched, fontID: tcmpj93mbl, obfuscatedNumber: 16570037
[2947432] 13:21:37 font downloaded
[2947432] 13:21:37 rendered image created
[2947432] 13:21:37 font recognized, mapping: 0169573842
[2947432] 13:21:37 real number parsed: 13780098
[2947432] 13:21:37 success
2020/07/02 13:21:37 all done in 539.568009ms
2020/07/02 13:21:37   total: 1
2020/07/02 13:21:37   success: 1
2020/07/02 13:21:37   error: 0
2020/07/02 13:21:37   ocr failed count: 0
$ cat result/result.txt
2947432,s5q2vdii2e,12980048,13780098
# 房间号，字体 ID，假数据，真数据
# 房间号为 2947432 的主播，关注人数为 13780098
```

结果存储在 `result` 目录中：

- `result/fonts`: 缓存所有下载的字体
- `result/mapping.json`: 缓存字体的映射关系
- `result/result.txt`: 存储爬取结果
- `result/tmp`: 存储临时文件，比如字体渲染以后的图片

爬取大量主播时，也可以通过文件来传递房间号，每行一个房间号。

```bash
$ douyu-crawler-demo -f roomids.txt
```

## OCR 识别字体

可以传入字体进行 OCR 识别得到结果。

```bash
$ douyu-crawler-demo -ocr fake.woff
ocr result: 8123456709
```

## 生成混淆字体用于反爬

传递基准字体文件名以及需要生成的混淆字体数量。

这里我们以 [Hack](https://sourcefoundry.org/hack/) 字体为例，注意使用前先记得裁剪。

```bash
$ cd douyu-crawler-demo
$ ./genfont.py hack.subset.ttf 20
....
$ ls result/generated # 结果存储在这个目录中
0018fb8365.7149586203.ttf  267ccb0e95.8402759136.ttf
08a9457ab9.3958406712.ttf  281ef45f09.2154786390.ttf
1bbdd405ca.9147328650.ttf  788e0c7651.8790526413.ttf
1f985cd725.6417320895.ttf  5433d36fde.1326570894.ttf
2d56def315.8962135047.ttf  6844549191.3597082416.ttf
6bd27a4bac.0658392147.ttf  a422833064.8416930752.ttf
6e337094a4.0754261839.ttf  c7f0591c38.5761804329.ttf
9a0e22d6ad.9173452860.ttf  d3269bd2ce.0384976152.ttf
9a407f17c1.8379426105.ttf  f97691cc25.1587964230.ttf
44a428c37d.3602974581.ttf  ffe2c54286.6894312057.ttf
```

文件名的命名规则为 `fontID.mapping.ttf`。也就是说如果使用 `0018fb8365.7149586203.ttf` 这个字体，那么 0 会被渲染成为 7。
