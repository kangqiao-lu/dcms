USE dcms;
CREATE TABLE `dcms_cronjob` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT 'job id',
  `name` varchar(200) NOT NULL DEFAULT 'DCMS' COMMENT 'job name',
  `create_user` varchar(30) NOT NULL DEFAULT 'DCMS-USER' COMMENT 'create by user',
  `executor` varchar(1000) NOT NULL DEFAULT 'ls' COMMENT 'executable file abs name',
  `executor_flags` varchar(1000) NOT NULL DEFAULT '' COMMENT 'exec args for executor',
  `signature` varchar(32) NOT NULL DEFAULT '' COMMENT 'executor md5签名',
  `runner` varchar(100) NOT NULL DEFAULT 'root' COMMENT '任务执行的系统用户,不允许为root',
  `timeout` bigint(20) NOT NULL DEFAULT '0' COMMENT '使用方设置超时时间,0为不超时',
  `timeout_trigger` int(11) NOT NULL DEFAULT '0' COMMENT '超时后触发操作:0 忽略, 1 KILL',
  `disabled` int(11) NOT NULL DEFAULT '0' COMMENT '当前job是否禁掉',
  `schedule` varchar(100) NOT NULL DEFAULT '* * */1 * *' COMMENT 'crontabl format',
  `hook` varchar(1000) NOT NULL DEFAULT '' COMMENT '任务执行后无论成功与否都会post数据到该webhook',
  `msg_filter` varchar(1000) NOT NULL DEFAULT '' COMMENT '任务输出,捕获filter关键词,以逗号分隔,如error,failed,critical',
  `create_at` int(11) NOT NULL DEFAULT '0' COMMENT 'job创建修改时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `name` (`name`),
  KEY `create_user` (`create_user`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8 COMMENT='crontab任务列表';

CREATE TABLE `dcms_agent2job` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT 'agent id',
  `host` varchar(100) NOT NULL DEFAULT '' COMMENT 'agent对应的主机名',
  `job_id` bigint(20) NOT NULL DEFAULT '0' COMMENT '当前agent所订阅的任务cronjob id',
  PRIMARY KEY (`id`),
  UNIQUE KEY `host` (`host`,`job_id`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8 COMMENT='agent订阅cronjob表';

CREATE TABLE `dcms_crontask_log` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT 'increment id',
  `job_id` bigint(20) NOT NULL DEFAULT '0' COMMENT 'cron job id',
  `task_id` varchar(100) NOT NULL DEFAULT '' COMMENT 'task id 字符串',
  `status` int(11) NOT NULL DEFAULT '0' COMMENT '0 ready,1 running, 2 success, 3 failed, 4 timeout, 5 killed',
  `exec_at` int(11) NOT NULL DEFAULT '0' COMMENT '任务开时执行时间戳',
  `exec_duration` int(11) NOT NULL DEFAULT '0' COMMENT '执行时长，单位秒',
  `log_filename` varchar(200) NOT NULL DEFAULT '' COMMENT 'agent宿主机日志全路径',
  PRIMARY KEY (`id`),
  KEY `job_id` (`job_id`),
  KEY `task_id` (`task_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 COMMENT='单次任务执行日志表';