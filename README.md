# 執行

## console application
### 未編譯運行 
* make console
* port預設8081
### Docker運行
#### 產生image
* `docker-build-console`
#### 運行container
* `docker-run-console`
#### 停止container
* `docker-stop-console`
  

## frontend application
### 未編譯運行
* make frontend ARGS="-p 8090"
* port預設8081
### Docker運行
#### 產生image
* `docker-build-frontend`
#### 運行container
* `docker-run-frontend`
#### 停止container
* `docker-stop-frontend`

## consumer application
### 未編譯運行
* make consumer
### Docker運行
#### 產生image
* `docker-build-consumer`
#### 運行container
* `docker-run-consumer`
#### 停止container
* `docker-stop-consumer`
#### 產生 2048 位元的 RSA 私鑰
* `openssl genrsa -out config/keys/private.pem 2048`
#### 從私鑰中提取公鑰
* `openssl rsa -in config/keys/private.pem -pubout -out config/keys/public.pem`

## TODO
* 可以考慮要不要把frontend跟console的預設port寫在config檔
* outbox若需要可以實作retry limit上限
* machineId可以改一下機制，如果不是使用stateful set的話
* 場次的capacity的實作
* index還沒有設定
* 訂單關鍵字搜尋，要看前端需要什麼條件再實作
* 目前遇到invalid message會直接dequeue，但是如果是特例的話可能要另外處理
* 可以定時清理zombie bill
* 當初因為趕時間所以權限認證放在controller
* 之後要整合發票系統，目前是一個訂單對應一張發票，看之後的需求是不是一個帳單對應一張發票