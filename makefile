.PHONY: all linux deploy

all:
	@mkdir -p bin/
	@bash --norc -i ./scripts/build.sh

run:
	@go run *.go --datadir=./data/ --log-level=debug

linux:
	@mkdir -p bin/
	@export GOOS=linux && export GOARCH=amd64 && bash --norc -i ./scripts/build.sh

deploy: linux
	@echo "--> Uploading..."
	scp -P 3389 contrib/init/peskar.default leo@paradev.ru:/etc/default/peskar
	scp -P 3389 contrib/init/peskar leo@paradev.ru:/etc/init.d/peskar
	scp -P 3389 bin/peskar-hub leo@paradev.ru:/opt/peskar/peskar_new
	@echo "--> Restarting..."
	ssh -p 3389 leo@paradev.ru service peskar stop
	ssh -p 3389 leo@paradev.ru rm /opt/peskar/peskar
	ssh -p 3389 leo@paradev.ru mv /opt/peskar/peskar_new /opt/peskar/peskar
	ssh -p 3389 leo@paradev.ru service peskar start
	@echo "--> Getting last logs..."
	@ssh -p 3389 leo@paradev.ru tail -n 25 /var/log/peskar.log

logs:
	@ssh -p 3389 leo@paradev.ru tail -n 200 /var/log/peskar.log

test:
	curl -X POST \
		-d '{"download_url": "http://tps91.homeftp.net/Martian_Extended.avi"}' \
		http://api.peskar.paradev.ru/job/
	curl -X POST \
		-d '{"download_url": "http://stormy.homeftp.net/Interstellar_IMAX_Edition.avi"}' \
		http://api.peskar.paradev.ru/job/
	curl -X POST \
		-d '{"download_url": "http://stormy.homeftp.net/Daap_hyut_cam_mui.avi"}' \
		http://api.peskar.paradev.ru/job/
