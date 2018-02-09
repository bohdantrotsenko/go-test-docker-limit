# testing memory limits in docker

The idea is to make an app/microservice check on its own whether a memory limit is set,
then monitor its activity and, on approaching, namely, 2/3 of the limit, return errors,
as opposed to reaching the limit and getting killed by OOM.

So far, a reproducible case was observed on ubuntu artful (17.10),
kernel 4.13.0-32, docker version 18.02.
Meaning, that the docker runtime indeed terminated the app on out-of-memory.

On Mac X 10.13.3 with docker 17.12,
on ubuntu xenial, kernel 4.4.0-112, docker versions 17.12, 18.02 (tried both of them),
the limit is not respected and the app went out of the memory limit without OOM.



