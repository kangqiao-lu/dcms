DCMS : distributed crontab management system

DCMS has two parts:

one for crontab by every machine

one for distributed jobs stateless

crontab集中管理平台和分布式任务调度平台

在固定机器上执行的是crontab
无状态的计算,不依赖机器的任务是jobs,当前仅实现crontab,并且只提供了接口,dashboard管理界面需要自行实现

安装
===

依赖:
* golang > 1.3
* linux redhat / centos
* mysql > 5.1


下载源码

	go get github.com/dongzerun/dcms

编译

	make

运行
===

	bin/dcms-agent  -h

	Usage of bin/dcms-agent:
	  -db="root:root@tcp(localhost)/dcms": mysql url used for jobs
	  -dbtype="mysql": store cron job db type
	  -port="8001": management port by http protocol
	  -quit_time=3600: when agent recevie, we wait quit_time sec for all TASK FINISHED
	  -verbose="info": log verbose:info, debug, warning, fatal
	  -work_dir="/tmp": work dir, used to save log file etc..


	Usage of bin/dcms-agent:
	  -db  			mysql 连接串
	  -dbtype 		存储类型, 当前接口仅实现mysql, 添加新DB类型只要实现接口即可
	  -port 		rest api 端口, 用于管理及dashboard集成
	  -quit_time	当接收sigterm信号后, agent等待所有任务退出, quit_time至期后 kill扔然运行进程
	  -verbose		日志级别, 用于调戏
	  -work_dir		工作目录, 建义好好归划


	导入表结构
	mysql -uroot -hlocalhost < agent/agent.sql

	创建工作目录
	mkdir /home/dcms

	运行agent
	bin/dcms-agent -db="root:root@tcp(localhost)dcms" -dbtype="mysql" -port="8001" -work_dir="/home/dcms" -quit_time=1800




