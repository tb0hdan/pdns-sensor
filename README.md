# pdns-sensor
Passive DNS Sensor - open source project for collecting passive DNS data from various sources and sending it to [DomainsProject.org](https://domainsproject.org).

*WARNING: This project is intended to be used only on your own hardware and network. Do not use it on other networks without permission!*

## Intro 

### Supported sources 

- TCPDump subprocess
- Mikrotik DNS logs (/var/log/network.log by default)

### Supported targets

- DomainsProject.org (public API)

## Running

Build the project:

```bash
make build
```

Run the project (requires sudo):

```bash
sudo build/pdns-sensor -enable-mikrotik
```

or 

```bash
sudo build/pdns-sensor -enable-tcpdump
```

or 

```bash
sudo build/pdns-sensor -enable-tcpdump -enable-mikrotik
```
