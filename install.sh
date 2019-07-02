#!/bin/bash

sudo touch /etc/https-forward
sudo systemctl enable $(realpath https-forward.service)
sudo mkdir -p /tmp/autocert
sudo chown nobody:users /tmp/autocert
sudo chmod 750 /tmp/autocert
