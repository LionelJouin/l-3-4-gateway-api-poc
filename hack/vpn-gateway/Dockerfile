FROM ubuntu:24.04

RUN apt-get update -y --fix-missing\
  && apt-get install -y iproute2 tcpdump iptables net-tools iputils-ping ipvsadm netcat-traditional wget bird2 ethtool conntrack xz-utils curl

WORKDIR /root/

ADD https://github.com/Nordix/ctraffic/releases/download/v1.7.0/ctraffic.gz ctraffic.gz
RUN gunzip ctraffic.gz \
  && chmod u+x ctraffic \
  && mv ctraffic /usr/bin/

ADD https://github.com/Nordix/mconnect/releases/download/v2.2.0/mconnect.xz mconnect.xz
RUN unxz mconnect.xz \
  && chmod u+x mconnect \
  && mv mconnect /usr/bin/ \
  && mkdir -p /etc/bird/ \
  && mkdir -p /run/bird

ARG BIRD_CONFIG_PATH=hack/vpn-gateway

COPY $BIRD_CONFIG_PATH/bird-common.conf /etc/bird/
COPY $BIRD_CONFIG_PATH/bird-gw.conf /etc/bird/
COPY $BIRD_CONFIG_PATH/bird-gw-no-default.conf /etc/bird/
COPY $BIRD_CONFIG_PATH/bird-filler-net.conf /etc/bird/

COPY $BIRD_CONFIG_PATH/init.sh /init.sh
RUN chmod +x /init.sh

CMD ["/init.sh", "100"]