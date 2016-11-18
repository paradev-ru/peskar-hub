.PHONY: all linux deploy

run:
	@go run *.go --datadir=./data/ --log-level=debug --dnd-enable=1 --dnd-start=1 --dnd-end=12

all:
	@mkdir -p bin/
	@bash --norc -i ./scripts/build.sh

linux:
	@mkdir -p bin/
	@export GOOS=linux && export GOARCH=amd64 && bash --norc -i ./scripts/build.sh

deploy: linux
	@echo "--> Uploading..."
	scp -P 3389 contrib/init/sysvinit-debian/peskar.default leo@paradev.ru:/etc/default/peskar
	scp -P 3389 contrib/init/sysvinit-debian/peskar leo@paradev.ru:/etc/init.d/peskar
	scp -P 3389 bin/peskar-hub leo@paradev.ru:/opt/peskar/peskar_new
	@echo "--> Restarting..."
	ssh -p 3389 leo@paradev.ru service peskar stop
	ssh -p 3389 leo@paradev.ru rm /opt/peskar/peskar
	ssh -p 3389 leo@paradev.ru mv /opt/peskar/peskar_new /opt/peskar/peskar
	ssh -p 3389 leo@paradev.ru service peskar start
	@echo "--> Getting last logs..."
	@ssh -p 3389 leo@paradev.ru tail -n 25 /var/log/peskar.log

logs:
	@ssh -p 3389 leo@paradev.ru tail -n 100 /var/log/peskar.log

test:
	@curl -X POST -d '{"download_url": "http://tps91.homeftp.net/Martian_Extended.avi"}' http://api.peskar.paradev.ru/job/
	@curl -X POST -d '{"download_url": "http://stormy.homeftp.net/Interstellar_IMAX_Edition.avi"}' http://api.peskar.paradev.ru/job/
	@curl -X POST -d '{"download_url": "http://stormy.homeftp.net/HD/720p/Fargo_BDRip_720p.mkv"}' http://api.peskar.paradev.ru/job/
	@curl -X POST -d '{"download_url": "http://megatron.is-very-evil.org/HD/720p/Rock_BDRip_720p.mkv"}' http://api.peskar.paradev.ru/job/
	@curl -X POST -d '{"download_url": "http://stormy.homeftp.net/HD/720p/Mission_Impossible_BDRip_720p.mkv"}' http://api.peskar.paradev.ru/job/
	@curl -X POST -d '{"download_url": "http://masterchief.is-a-chef.com/HD/720p/Nutty_Professor_HDDVDRip_720p.mkv"}' http://api.peskar.paradev.ru/job/
	@curl -X POST -d '{"download_url": "http://megatron.is-very-evil.org/HD/720p/Dont_Be_a_Menace_to_South_Central_While_Drinking_Your_Juice_in_the_Hood_BDRip_720p.mkv"}' http://api.peskar.paradev.ru/job/
	@curl -X POST -d '{"download_url": "http://stormy.homeftp.net/HD/HDTV/Tin_Cup_HDTVRip_720p.mkv"}' http://api.peskar.paradev.ru/job/
	@curl -X POST -d '{"download_url": "http://megatron.is-very-evil.org/HD/720p/Scream_BDRip_720p.mkv"}' http://api.peskar.paradev.ru/job/
	@curl -X POST -d '{"download_url": "http://stormy.homeftp.net/HD/1080p/Neon_Demon_BDRip_1080p.mkv"}' http://api.peskar.paradev.ru/job/
	@curl -X POST -d '{"download_url": "http://stormy.homeftp.net/HD/HDTV/Mechanic_Resurrection_WEB-DL_1080p.mkv"}' http://api.peskar.paradev.ru/job/
