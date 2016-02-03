#RUN

go build && sudo ./deecper --log-level debug --scope global --no-ipam

#USE

docker network create --driver dnet --opt iface=enp0s8 --subnet=192.168.72.0/24 --gateway=192.168.72.1 --aux-address u1=192.168.72.5

