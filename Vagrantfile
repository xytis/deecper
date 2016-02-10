# -*- mode: ruby -*-
# vi: set ft=ruby :

Vagrant.configure(2) do |config|
  config.vm.provider :virtualbox do |vb|
    vb.customize ["modifyvm", :id, "--memory", "256"]
  end

  config.vm.define vm_name = "default" do |config|
    config.vm.box = "centos/7g"
    config.vm.synced_folder "~/Documents/leos/go", "/home/vagrant/go"
    config.vm.network "private_network", ip: "192.168.72.5"
    config.vm.provider :virtualbox do |vb|
      vb.customize ["modifyvm", :id, "--memory", "2048"]
    end
  end

  config.vm.define vm_name = "dhcp" do |config|
    config.vm.hostname = "dhcpd.net"
    config.vm.box = "debian/jessie64"
    config.vm.network "private_network", ip: "192.168.72.254"
    config.vm.provision "ansible" do |ansible|
      ansible.verbose = "vvvv"
      ansible.playbook = "/Users/xytis/Documents/leos/site/deploy-master/linux.dhcpd.yml"
      ansible.extra_vars = {
        "dhcpd_net" => "192.168.72.0/24"
      }
      ansible.groups = {
        "linuxdhcpd" => ["dhcp"]
      }
    end
  end

  config.vm.define vm_name = "client" do |config|
    config.vm.box = "debian/jessie64"
    config.vm.network "private_network", type: "dhcp", ip: "192.168.72.5"
  end
end
