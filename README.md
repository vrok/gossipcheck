# gossipcheck

Gossipcheck is a tool that runs checks on a cluster using a gossip-like protocol. A command can be run in response to a failing check. Thanks to the gossip-style communication, it is resilient and easily scalable.

It is based on the same library that is used in Serf (called memberlist), which implements the SWIM protocol (with some modifications).

Unlike Serf, gossipcheck doesn't piggyback its messages in SWIM datagrams. That's because they are can be large and wouldn't fit into UDP packets. `memberlist` is used mostly for handling membership changes, `gossipcheck` actually uses its own SWIM-like layer on top of `memberlist`. The biggest difference is that - since we're not piggybacking on SWIM messages (that are send with a fixed frequency) - when a new batch of jobs appears, we can distribute it as fast as possible. It is called the "burst" phase. It is still possible that the batch won't reach a small number of nodes in the first phase (though with a good gossip group size the chance is minimal), but then there's the second phase that's exactly like SWIM, that eventually delivers the message everywhere.

## Installation

```
go get github.com/vrok/gossipcheck/...
```

Run `gcheck -h` and `gossipcheckd -h` for information about available parameters.

```
> gossipcheckd -h
Usage of gossipcheckd:
  -bind string
    	The address used for communication with other nodes (default ":3505")
  -cli-bind string
    	The address where CLI client connects to (default "127.0.0.1:5924")
  -gossip-group int
    	Number of nodes that this node will talk to in every iteration (default 5)
  -no-cli
    	Don't start command line RPC server
  -peers string
    	Comma-separated list of addresses of initial peers (empty for the first node)
      
> gcheck -h
Usage of gcheck:
  -file string
    	JSON file with checks to run
  -server string
    	The address where CLI client connects to (default "127.0.0.1:5924")
```

A simple tutorial is below.

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

If your cluster consisted of millions of nodes, then you would probably want to make sure that the checks that you're about to distribute are correct. Fortunately, it is possible to run them just on the local node with the `local-check` command (they are executed sequentially, and it doesn't run actions, so that debugging is easier):

```
./gcheck -file=file.json local-check 
```
