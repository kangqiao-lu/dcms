DCMS : distributed crontab management system

DCMS has two parts:

one for crontab by every machine

one for distributed jobs stateless

crontab集中管理平台和分布式任务调度平台

随着公司规模增长, 原有部署在单机crontab的任务错乱复杂, 不方便运维及开发同学部署, 使用, 查看及管理.需要一个集中式管理平台, 代替原有单机实现, 同时提供更多的任务管理功能.

对于无状态的不依赖机器的crontab任务, 需要分布式调度, DCMS目标也在于此. 当前市面上有 python celery, c gearman 等任务调度实现, 简单可依赖, 适合小公司快速上手, 同时缺点也很明显, 对调度的任务管控能力有限, 并且无法将资源粒度控制到mem, cpu, disk. 第二部份计划基于mesos实现分布式调度。

在固定机器上执行的是crontab, 无状态的计算, 不依赖机器的任务是jobs, 当前仅实现crontab,并且只提供了接口, dashboard管理界面暂时需要使用者自行实现.



功能及规划
===

功能:
* 单机类`crontab`任务调度.
* 支持任务的`md5`签名, 防止任务脚本被恶意变更.
* 支持任务状态的回传数据库, 可配置的`webhook`, 将状态post过去.
* 支持对超时任务的默认处理.
* 支持远程`kill`正在运行任务.
* 支持`rest-api`管理当前`agent`任务.
* 支持对`crontab`的输出进行关键词过滤, 如`fatal``error``fail`等, 以管道符`|`分隔.

约束:
* 任务脚本必须有可执行权限, 即 `chmod u+x exec_file`.
* 任务不允许以`root`运行, 必须是普通用户.

规划:
* 实现基于`mesos`的分布式任务调度.
* 简单开箱即用的`dashboard`管理界面.



安装
===

依赖:
* golang > 1.3
* linux redhat / centos
* mysql > 5.1


下载源码

	go get github.com/dongzerun/dcms

	或

	git clone https://github.com/dongzerun/dcms

编译

	cd github.com/dongzerun/dcms; make




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

	运行agent(可以用nohup或supervise运行)
	bin/dcms-agent -db="root:root@tcp(localhost)dcms" -dbtype="mysql" -port="8001" -work_dir="/home/dcms" -quit_time=1800


测试
===

准备脚本

	cat /home/dcms/test.sh
	#!/bin/sh
	echo "output failed ? sucess ? or error ?"
	sleep 40
	echo "test run done"

	chmod u+x /home/dcms/test.sh

	md5 /tmp/test.sh 
	MD5 (/tmp/test.sh) = 88eb1e5ec830e60ac77fdf869962b928

配置任务

	将`crontab`任务添加到DB中

	insert into dcms_cronjob (name, create_user, executor, executor_flags, signature,runner, timeout, timeout_trigger, disabled, schedule, hook, msg_filter, create_at) values ("jobtestname","dongzerun","/home/dcms/test.sh","","88eb1e5ec830e60ac77fdf869962b928","dzr",30,0,0,"*/1 * * * *", "http://testwebhook.com","fatal|failed|error",unix_timestamp());


	将`agent`机器以`hostname`和`job_id`进行关联注册
	insert into dcms_agent2job (host, job_id) values ('localhost', cronjob_pk_id);





