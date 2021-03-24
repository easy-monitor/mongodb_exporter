# MongoDB exporter

## 声明
基于开源项目mongodb_exporter
https://github.com/percona/mongodb_exporter
该exporter只能采集一个目标，不能动态指定采集目标
要想更改采集目标需要重新启动
启动形式
```
exporter --mongodb.uri=mongodb://127.0.0.1:27017
```


## 目的：
保留原有功能情况下，将exporter改造成可以动态指定采集目标，做到多目标采集

## 用法：
在项目根路径下的conf.yml配置文件中配置mongodb的信息
```
default-target:
  host: 127.0.0.1
  port: 27017

module:
  - name: mongo
    user: root
    password: root
  - name: mongo-1
    user: root-11
    password: root-1
```
原有启动形式启动
```
exporter --mongodb.uri=mongodb://127.0.0.1:27017
```
可以通过url动态指定采集目标
```
#指定mongodb和module，会在conf.yml文件中获取账号密码模块
http://127.0.0.1:9216/metrics?target={{ip}}:{{port}}&module={{module-name}}

#指定mongodb不指定module，以不指定账号密码形式采集
http://127.0.0.1:9216/metrics?target={{ip}}:{{port}}

#当不指定target时，以conf.yml所指定的default-target目标作为采集目标
http://127.0.0.1:9216/metrics
```

