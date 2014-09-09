# nik - a stupid Docker hostfile updater

You want your containers to know about each other? You got it. For the low low price of a couple of CPU cycles, I will happily inject the names and IP addresses of all your containers into all your other containers! How cool is that?

Of course, I am very stupid and you should not use me in production. What's that? You **will* run me in production? Oh, I'm tickled pink. Let's hope I don't leak memory or something.

## Quickstart

Start up the updater process:

	$ ./nik

Now, in a separate console, try:

	$ docker run -d --hostname apple busybox sleep 600
	$ docker run -d --hostname pear busybox sleep 600
	$ docker run -ti --rm --hostname userguy busybox /bin/sh

You should be able to ping `pear` and `apple`. Your `/etc/hosts` should have both entries.

For bonus points, in yet another console do:

	$ docker run -d --hostname watermelon busybox sleep 600

Now ping `watermelon` from your `userguy` shell.

## Details

  * all containers are injected with their `Hostname`, make sure to set it using `--hostname`
  * the `-i` flag controls the check interval