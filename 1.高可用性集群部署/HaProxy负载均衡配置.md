# 配置 HaProxy 作为 apiserver 的负载均衡

## 三个 Kubernetes 控制节点编译安装 Haproxy

安装编译所需软件包

    yum install gcc pcre-static pcre-devel systemd-devel -y
下载 haproxy

    wget https://www.haproxy.org/download/1.8/src/haproxy-1.8.13.tar.gz

编译安装

    tar xzvf haproxy-1.8.13.tar.gz
    cd haproxy-1.8.13
    make TARGET=linux2628 USE_SYSTEMD=1
    make install
    mkdir -p /etc/haproxy
    mkdir -p /var/lib/haproxy

配置 haproxy 服务启动配置文件

    vi /usr/lib/systemd/system/haproxy.service
    [Unit]
    Description=HAProxy Load Balancer
    After=network.target

    [Service]
    Environment="CONFIG=/etc/haproxy/haproxy.cfg" "PIDFILE=/run/haproxy.pid"
    ExecStartPre=/usr/local/sbin/haproxy -f $CONFIG -c -q
    ExecStart=/usr/local/sbin/haproxy -Ws -f $CONFIG -p $PIDFILE
    ExecReload=/usr/local/sbin/haproxy -f $CONFIG -c -q
    ExecReload=/bin/kill -USR2 $MAINPID
    KillMode=mixed
    Restart=always
    SuccessExitStatus=143
    Type=notify

    # The following lines leverage SystemD's sandboxing options to provide
    # defense in depth protection at the expense of restricting some flexibility
    # in your setup (e.g. placement of your configuration files) or possibly
    # reduced performance. See systemd.service(5) and systemd.exec(5) for further
    # information.

    # NoNewPrivileges=true
    # ProtectHome=true
    # If you want to use 'ProtectSystem=strict' you should whitelist the PIDFILE,
    # any state files and any other files written using 'ReadWritePaths' or
    # 'RuntimeDirectory'.
    # ProtectSystem=true
    # ProtectKernelTunables=true
    # ProtectKernelModules=true
    # ProtectControlGroups=true
    # If your SystemD version supports them, you can add: @reboot, @swap, @sync
    # SystemCallFilter=~@cpu-emulation @keyring @module @obsolete @raw-io

    [Install]
    WantedBy=multi-user.target

配置 haproxy

    vi /etc/haproxy/haproxy.cfg
    global
        log /dev/log local0
        log /dev/log local1 notice
        chroot /var/lib/haproxy
        stats timeout 30s
        maxconn 5000
        daemon

    defaults
        log global
        mode http
        option dontlognull
        timeout connect 5000
        timeout client 50000
        timeout server 50000

    frontend k8s-api
        bind 0.0.0.0:6443
        mode tcp
        default_backend k8s-api

    backend k8s-api
        mode tcp
        option tcp-check
        balance roundrobin
        default-server inter 10s downinter 5s rise 2 fall 2 slowstart 60s maxconn 250 maxqueue 256 weight 100
        server k8s-api-1 192.168.115.83:6443 check
        server k8s-api-2 192.168.115.84:6443 check
        server k8s-api-3 192.168.115.85:6443 check

启动 HaProxy 并设置开机自启动

    systemctl start haproxy
    systemctl enable haproxy

## 配置 keepalived 服务

安装 keepalived

    yum install -y keepalived psmisc

节点1上配置

    vi /etc/keepalived/keepalived.conf
    global_defs {
    notification_email {
        root@localhost
    }
    notification_email_from admin@allen.com
    smtp_server 127.0.0.1
    smtp_connect_timeout 30
    router_id LVS_ALLEN
    }
    vrrp_script haproxy-check {
        script "killall -0 haproxy"
        interval 2
        weight -30
    }
    vrrp_instance k8s-api {
        state MASTER
        interface eth0
        virtual_router_id 100
        priority 100
        advert_int 1
        authentication {
            auth_type PASS
            auth_pass k8s
        }
        virtual_ipaddress {
            192.168.115.250
        }
        track_script {
            haproxy-check
        }
    }

节点2上配置：

    vi /etc/keepalived/keepalived.conf
    global_defs {
    notification_email {
        root@localhost
    }
    notification_email_from admin@allen.com
    smtp_server 127.0.0.1
    smtp_connect_timeout 30
    router_id LVS_ALLEN
    }
    vrrp_script haproxy-check {
        script "killall -0 haproxy"
        interval 2
        weight -30
    }
    vrrp_instance k8s-api {
        state BACKUP
        interface eth0
        virtual_router_id 100
        priority 90
        advert_int 1
        authentication {
            auth_type PASS
            auth_pass k8s
        }
        virtual_ipaddress {
            192.168.115.250
        }
        track_script {
            haproxy-check
        }
    }

节点3上配置：

    vi /etc/keepalived/keepalived.conf
    global_defs {
    notification_email {
        root@localhost
    }
    notification_email_from admin@allen.com
    smtp_server 127.0.0.1
    smtp_connect_timeout 30
    router_id LVS_ALLEN
    }
    vrrp_script haproxy-check {
        script "killall -0 haproxy"
        interval 2
        weight -30
    }
    vrrp_instance k8s-api {
        state BACKUP
        interface eth0
        virtual_router_id 100
        priority 80
        advert_int 1
        authentication {
            auth_type PASS
            auth_pass k8s
        }
        virtual_ipaddress {
            192.168.115.250
        }
        track_script {
            haproxy-check
        }
    }

各节点启动 keepalived

    systemctl start keepalived
    systemctl enable keepalived

## 验证可用性

VIP（192.168.115.250）开始绑定在控制节点1上

    [root@k8s-m1 ~]# ip a l
    1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN qlen 1
        link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
        inet 127.0.0.1/8 scope host lo
        valid_lft forever preferred_lft forever
        inet6 ::1/128 scope host
        valid_lft forever preferred_lft forever
    2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP qlen 1000
        link/ether fa:16:3e:44:b9:61 brd ff:ff:ff:ff:ff:ff
        inet 192.168.115.83/24 brd 192.168.115.255 scope global dynamic eth0
        valid_lft 540sec preferred_lft 540sec
        inet 192.168.115.250/32 scope global eth0
        valid_lft forever preferred_lft forever
        inet6 fe80::f816:3eff:fe44:b961/64 scope link
        valid_lft forever preferred_lft forever
    3: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN
        link/ether 02:42:52:3d:25:02 brd ff:ff:ff:ff:ff:ff
        inet 172.77.1.1/24 scope global docker0
        valid_lft forever preferred_lft forever

停掉节点1上的 haproxy 服务，VIP已经飘走

    [root@k8s-m1 ~]# systemctl stop haproxy
    [root@k8s-m1 ~]# ip a l
    1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN qlen 1
        link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
        inet 127.0.0.1/8 scope host lo
        valid_lft forever preferred_lft forever
        inet6 ::1/128 scope host
        valid_lft forever preferred_lft forever
    2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP qlen 1000
        link/ether fa:16:3e:44:b9:61 brd ff:ff:ff:ff:ff:ff
        inet 192.168.115.83/24 brd 192.168.115.255 scope global dynamic eth0
        valid_lft 525sec preferred_lft 525sec
        inet 192.168.115.250/32 scope global eth0
        valid_lft forever preferred_lft forever
        inet6 fe80::f816:3eff:fe44:b961/64 scope link
        valid_lft forever preferred_lft forever
    3: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN
        link/ether 02:42:52:3d:25:02 brd ff:ff:ff:ff:ff:ff
        inet 172.77.1.1/24 scope global docker0
        valid_lft forever preferred_lft forever

VIP已经绑定到节点2上

    [root@k8s-m2 ~]# ip a l
    1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN qlen 1
        link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
        inet 127.0.0.1/8 scope host lo
        valid_lft forever preferred_lft forever
        inet6 ::1/128 scope host
        valid_lft forever preferred_lft forever
    2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP qlen 1000
        link/ether fa:16:3e:90:af:3e brd ff:ff:ff:ff:ff:ff
        inet 192.168.115.84/24 brd 192.168.115.255 scope global dynamic eth0
        valid_lft 489sec preferred_lft 489sec
        inet 192.168.115.250/32 scope global eth0
        valid_lft forever preferred_lft forever
        inet6 fe80::f816:3eff:fe90:af3e/64 scope link
        valid_lft forever preferred_lft forever
    3: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN
        link/ether 02:42:f4:c9:6d:fa brd ff:ff:ff:ff:ff:ff
        inet 172.77.1.1/24 scope global docker0
        valid_lft forever preferred_lft forever

停掉节点2上的 haproxy 服务，VIP绑定到节点3上

    [root@k8s-m2 ~]# systemctl stop haproxy

    [root@k8s-m3 ~]# ip a l
    1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN qlen 1
        link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
        inet 127.0.0.1/8 scope host lo
        valid_lft forever preferred_lft forever
        inet6 ::1/128 scope host
        valid_lft forever preferred_lft forever
    2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP qlen 1000
        link/ether fa:16:3e:d7:3b:08 brd ff:ff:ff:ff:ff:ff
        inet 192.168.115.85/24 brd 192.168.115.255 scope global dynamic eth0
        valid_lft 512sec preferred_lft 512sec
        inet 192.168.115.250/32 scope global eth0
        valid_lft forever preferred_lft forever
        inet6 fe80::f816:3eff:fed7:3b08/64 scope link
        valid_lft forever preferred_lft forever
    3: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN
        link/ether 02:42:a8:0c:2e:a1 brd ff:ff:ff:ff:ff:ff
        inet 172.77.1.1/24 scope global docker0
        valid_lft forever preferred_lft forever

启动节点1的haproxy，VIP重新绑定到节点1

    [root@k8s-m1 ~]# systemctl start haproxy
    [root@k8s-m1 ~]# ip a l
    1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN qlen 1
        link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
        inet 127.0.0.1/8 scope host lo
        valid_lft forever preferred_lft forever
        inet6 ::1/128 scope host
        valid_lft forever preferred_lft forever
    2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP qlen 1000
        link/ether fa:16:3e:44:b9:61 brd ff:ff:ff:ff:ff:ff
        inet 192.168.115.83/24 brd 192.168.115.255 scope global dynamic eth0
        valid_lft 452sec preferred_lft 452sec
        inet 192.168.115.250/32 scope global eth0
        valid_lft forever preferred_lft forever
        inet6 fe80::f816:3eff:fe44:b961/64 scope link
        valid_lft forever preferred_lft forever
    3: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN
        link/ether 02:42:52:3d:25:02 brd ff:ff:ff:ff:ff:ff
        inet 172.77.1.1/24 scope global docker0
        valid_lft forever preferred_lft forever

> 在 Kubernetes 集群搭建时使用 HaProxy 提供的 VIP（192.168.115.250）和端口（6443）就可以了。
