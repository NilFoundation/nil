Test environment for debugging the interaction of multiple nodes via libp2p with a specified network topology.

Based on [libp2p/test-plans](https://github.com/libp2p/test-plans), in particular the Router is taken as is,
but significantly simplified and reduced to the necessary minimum.

Currently one scenario is configured:
- Two unconnected subnets (`lan_listener` and `lan_dialer`), connected through `router`s to a third one (`internet`)
- A `relay` located in `internet`
- A `listener` connected to the `relay` from the subnet `lan_listener`
- A `dialer` trying to send a request to the `listener` through the `relay`

Orchestration (passing node addresses between each other) is carried out
through a `registry` key-value store (Redis instance), connected to all subnets.

## Usage
### Building
```
docker compose build
```

### Deployment
```
docker compose up
```

or

```
docker compose up -d
docker compose logs <container>
```

### Expectations
After some time following the launch, we should see in the `dialer` logs a message similar to:
```
dialer-1  | 2025/05/06 12:46:42 ping: RTT=250.709244ms (error=<nil>)
```

This means that the ping from the `dialer` in `lan_dialer` reached the `listener` in `lan_listener` through the `relay`.
The RTT consists of delays of 100 ms on the routers (see `DELAY_MS` in `docker-compose.yml`)
and 2x25ms delays on the relay (see the `relay` configuration in `docker-compose.yml`).

If we manage to set up hole-punching, it is expected that instead of 250 we will get 200,
as it happens in `test-plans/hole-punch-interop`.
