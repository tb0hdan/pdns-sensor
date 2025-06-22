# pdns-sensor
Passive DNS Sensor

## Intro 

### Supported sources 

- TCPDump subprocess
- Mikrotik DNS logs

### Supported targets

- DomainsProject.org (public API)

## Running

Build the project:

```bash
make
```

Run the project:

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
