alter table apps
	add column auto_deploy_branch text not null default '';

alter table apps
	add column webhook_secret text not null default '';
