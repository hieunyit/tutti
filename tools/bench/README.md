# Tutti benchmark tool
See [config](pkg/config/conf.go). 

- The tool writes objects of **configured size** to **configured bucket** to TuttiProxy with **configured concurrency**.
- Write benchmarks logged to `CSV` file along with tutti meta.
- Every `N` writes the tool performs `LIST` and `GET` object against TuttiProxy and directly to main storage.
- Proxy and Main `LIST` and `GET` results also logged to a separate `CSV` files along with Tutti meta.

This allows to see how TuttiProxy behaves dependent on number of objects in bucket in comparison to Main storage.

Other features:
- The tool stores stated in local KV store on disk `tutti-bench.db` and will catch up from the same place in case of restart.



### Build binary for linux
```shell
GOOS=linux GOARCH=amd64 go build -o bench-linux-64 .
```

### Run in background
```shell
nohup bench-linux-64 > tutti_bench.log &
```
Will start benchmark in background and redirect logs into `tutti_bench.log` file.
Benchmarks results will appear in `CSV` files `bench_*_<obj_size>_P<concurrency>_<timestamp>.csv` in current directory.

