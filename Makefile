cycle: build-docker run-docker

build-docker:
	docker build . -t docker-mem-test

run-docker:
	docker run --rm -it -v `pwd`/data:/data -m 200m --cpus=1 docker-mem-test
