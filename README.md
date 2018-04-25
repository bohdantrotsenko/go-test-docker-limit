# testing memory limits in docker

The idea is to make an app/microservice check on its own whether a memory limit is set,
then monitor its activity and, on approaching, namely, 2/3 of the limit, return errors,
as opposed to reaching the limit and getting killed by OOM.

To demonstrate the idea I create a small program that does "work".
For the purposes of the demo the work is to allocate some memory (like, 1MiB) and to sleep for some time ~t.
Each "work item" is introduced N times per second.

This is a stochastic process that tends to consume N * t * size memory.

The memory watchdog I introduce is a hook for GC, a finalizer for some object. Having it enables me to know
when memory usage changes.

I couldn't came up with a better idea than to stop doing "work" when some kind of a soft memory limit is met.

I still couldn't figure out a plausible idea of detecting a hard memory limit one cannot detect due to OOM killer.

Feel free to play with code, chaging constants, uncommenting debug output, or turning "intelligence" off by renaming
`func init()`.
