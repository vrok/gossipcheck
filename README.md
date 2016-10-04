# gossipcheck

Gossipcheck is a tool that runs checks on a cluster using a gossip-like protocol. A command can be run in response to a failing check.

It is based on the same library that is used in Serf, which implements the SWIM protocol (with some modifications).

## Checks

Currently, there are 3 types of checks, which let you:

- check if a file exists
- check if a file contains a given text
- check if a process is running

They are specified with simple JSON files, for example:

```json
{
  "resolv_has_8888": {
    "type": "file_contains",   
    "path": "/etc/resolv.conf", 
    "check": "8.8.8.8",
    "action": "echo 8.8.8.8 >> /etc/resolv.conf"
  }, 
  "syslog_pidfile_exists": {
    "type": "file_exists",
    "path": "/var/run/syslog.pid", 
    "action": "shutdown -r now"
  },
  "check_nginx_running": {
    "type": "process_running", 
    "path": "/usr/bin/nginx",
    "action": "service nginx restart"
  }
}
```

A word on `process_running` checks: the `path` field is used to match the executable location, but you can also use `action` to check if the command used to start it contains the given text (if both are specified, they are ANDed). And it only works on systems with procfs.

## Setting up

Gossipchecks has two binaries, `gossipcheckd` and `gcheck`. The former is a node daemon, that joins the cluster and then runs incoming checks. The latter is a command line utility that is used to control the daemon (usually the one that runs locally), and this is what you use to load a JSON and send it to the cluster.

Okay, let's setup a tiny cluster of 2 nodes running on the same host. The first one can be started like this:

```
./gossipcheckd -bind 127.0.0.1:2000
```

The second node needs a different command. To be able to join the cluster, a node needs to know addresses of some of the nodes that are already in it (at least one). We're also starting a second node on the same host as the first one, which means that the RPC server will try to bind to the same port - but we actually don't need it, so it can be disabled. The command that does all these things looks like this:

```
./gossipcheckd -no-cli -bind 127.0.0.1:2001 -peers 127.0.0.1:2000 
```

Now we have a local cluster of two nodes, and can run some checks on it:

```
./gcheck -file=file.json check
```

If your cluster consisted of millions of nodes, then you would probably want to make sure that the checks that you're about to distribute are correct. Fortunately, it is possible to run them just on the local node:

```
./gcheck -file=file.json local-check 
```
