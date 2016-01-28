# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure(2) do |config|
  config.vm.box = "centos/7g"
  config.vm.synced_folder "~/Documents/leos/go", "/home/vagrant/go"
  config.vm.network "private_network", ip: "192.168.72.2"
end
