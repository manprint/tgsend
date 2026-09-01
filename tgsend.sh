#!/bin/sh

set -eu

if ! command -v docker >/dev/null 2>&1; then
	printf '%s\n' 'tgsend: docker is required' >&2
	exit 127
fi

image=${TGSEND_IMAGE:-ghcr.io/manprint/tgsend:latest}
home=${HOME-}
workdir=${PWD-}
if [ -z "$workdir" ]; then
	workdir=$(pwd -P)
fi

config_path=
explicit_config=0

usage_error() {
	printf '%s\n' '{"schema_version":"1","ok":false,"command":"send","error":{"code":"invalid_flag","message":"invalid command-line flag","retryable":false}}' >&2
	exit 2
}

config_error() {
	case "$1" in
		config_not_found)
			message='configuration file not found'
			;;
		config_unreadable)
			message='configuration file is not readable'
			;;
		*)
			message='configuration is invalid'
			;;
	esac
	printf '{"schema_version":"1","ok":false,"command":"send","error":{"code":"%s","message":"%s","retryable":false}}\n' "$1" "$message" >&2
	exit 3
}

scan_config_flags() {
	expecting_value=0
	options_ended=0
	for argument do
		if [ "$options_ended" -eq 1 ]; then
			continue
		fi
		if [ "$expecting_value" -eq 1 ]; then
			if [ -z "$argument" ]; then
				usage_error
			fi
			case "$argument" in
				-*) usage_error ;;
			esac
			config_path=$argument
			explicit_config=1
			expecting_value=0
			continue
		fi
		case "$argument" in
			--)
				options_ended=1
				;;
			-c|--config)
				expecting_value=1
				;;
			--config=*)
				config_path=${argument#--config=}
				[ -n "$config_path" ] || usage_error
				explicit_config=1
				;;
			-c?*)
				config_path=${argument#-c}
				explicit_config=1
				;;
		esac
	done
	[ "$expecting_value" -eq 0 ] || usage_error
}

physical_path() {
	path=$1
	case "$path" in
		*/*)
			directory=${path%/*}
			filename=${path##*/}
			[ -n "$directory" ] || directory=/
			;;
		*)
			directory=.
			filename=$path
			;;
	esac
	case "$directory" in
		-*) directory=./$directory ;;
	esac
	physical_directory=$(CDPATH=; cd -P "$directory" 2>/dev/null && pwd -P) || return 1
	resolved=$physical_directory/$filename
	if command -v realpath >/dev/null 2>&1; then
		realpath "$resolved" 2>/dev/null || printf '%s\n' "$resolved"
	else
		printf '%s\n' "$resolved"
	fi
}

validate_config() {
	path=$1
	if [ ! -f "$path" ]; then
		if [ -d "$path" ] || [ -r "$path" ]; then
			config_error config_unreadable
		fi
		config_error config_not_found
	fi
	[ -r "$path" ] || config_error config_unreadable
	config_path=$(physical_path "$path") || config_error config_unreadable
}

scan_config_flags "$@"
if [ "$explicit_config" -eq 1 ]; then
	validate_config "$config_path"
elif [ -n "$home" ]; then
	default_config=$home/.tgsend
	if [ -f "$default_config" ]; then
		validate_config "$default_config"
	elif [ -d "$default_config" ] || [ -r "$default_config" ]; then
		config_error config_unreadable
	fi
fi

if [ "${TGSEND_TOKEN+x}" = x ]; then
	token_set=1
else
	token_set=0
fi
if [ "${TGSEND_CHAT_ID+x}" = x ]; then
	chat_id_set=1
else
	chat_id_set=0
fi

run_container() {
	if [ -n "$config_path" ]; then
		mount_arg="type=bind,src=$config_path,dst=$config_path,readonly"
		if [ "$token_set" -eq 1 ]; then
			if [ "$chat_id_set" -eq 1 ]; then
				exec docker run --rm -i --user "$(id -u):$(id -g)" --workdir "$workdir" --env "HOME=$home" --env TGSEND_TOKEN --env TGSEND_CHAT_ID --mount "$mount_arg" "$image" "$@"
			fi
			exec docker run --rm -i --user "$(id -u):$(id -g)" --workdir "$workdir" --env "HOME=$home" --env TGSEND_TOKEN --mount "$mount_arg" "$image" "$@"
		fi
		if [ "$chat_id_set" -eq 1 ]; then
			exec docker run --rm -i --user "$(id -u):$(id -g)" --workdir "$workdir" --env "HOME=$home" --env TGSEND_CHAT_ID --mount "$mount_arg" "$image" "$@"
		fi
		exec docker run --rm -i --user "$(id -u):$(id -g)" --workdir "$workdir" --env "HOME=$home" --mount "$mount_arg" "$image" "$@"
	fi
	if [ "$token_set" -eq 1 ]; then
		if [ "$chat_id_set" -eq 1 ]; then
			exec docker run --rm -i --user "$(id -u):$(id -g)" --workdir "$workdir" --env "HOME=$home" --env TGSEND_TOKEN --env TGSEND_CHAT_ID "$image" "$@"
		fi
		exec docker run --rm -i --user "$(id -u):$(id -g)" --workdir "$workdir" --env "HOME=$home" --env TGSEND_TOKEN "$image" "$@"
	fi
	if [ "$chat_id_set" -eq 1 ]; then
		exec docker run --rm -i --user "$(id -u):$(id -g)" --workdir "$workdir" --env "HOME=$home" --env TGSEND_CHAT_ID "$image" "$@"
	fi
	exec docker run --rm -i --user "$(id -u):$(id -g)" --workdir "$workdir" --env "HOME=$home" "$image" "$@"
}

run_container "$@"
