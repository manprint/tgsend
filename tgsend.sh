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
config_source=
config_target=

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

# scan_config_flags mirrors how the binary's flag parser reads -c/--config:
# a separate value is taken verbatim, even when it is empty or starts with a
# dash, and the values of the other value-taking flags are never inspected.
scan_config_flags() {
	expecting_value=0
	skip_value=0
	options_ended=0
	for argument do
		if [ "$options_ended" -eq 1 ]; then
			continue
		fi
		if [ "$skip_value" -eq 1 ]; then
			skip_value=0
			continue
		fi
		if [ "$expecting_value" -eq 1 ]; then
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
				explicit_config=1
				;;
			-c=*)
				config_path=${argument#-c=}
				explicit_config=1
				;;
			-c?*)
				config_path=${argument#-c}
				explicit_config=1
				;;
			-m|--message|--title|--type|--max-input-bytes)
				skip_value=1
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

# logical_path returns the absolute path the binary computes inside the
# container, where the working directory is the caller's logical $PWD.
logical_path() {
	case "$1" in
		/*) printf '%s\n' "$1" ;;
		*) printf '%s/%s\n' "${workdir%/}" "$1" ;;
	esac
}

# mount_field quotes one --mount field when the path would break Docker's
# comma-separated mount syntax.
mount_field() {
	case "$1" in
		*,*|*\"*) printf '"%s"' "$(printf '%s' "$1" | sed 's/"/""/g')" ;;
		*) printf '%s' "$1" ;;
	esac
}

# select_config mounts an existing regular file read-only. The source is the
# physical file, and the destination is the path the binary opens, so a
# symlinked config is found where the binary looks for it. A missing file is
# not mounted, which lets the binary report it exactly as a native run does.
select_config() {
	path=$1
	if [ ! -f "$path" ]; then
		if [ -d "$path" ] || [ -r "$path" ]; then
			config_error config_unreadable
		fi
		return 0
	fi
	[ -r "$path" ] || config_error config_unreadable
	config_source=$(physical_path "$path") || config_error config_unreadable
	config_target=$(logical_path "$path")
	mount_arg="type=bind,$(mount_field "src=$config_source"),$(mount_field "dst=$config_target"),readonly"
}

scan_config_flags "$@"
mount_arg=
if [ "$explicit_config" -eq 1 ]; then
	if [ -n "$config_path" ]; then
		select_config "$config_path"
	fi
elif [ -n "$home" ]; then
	select_config "$home/.tgsend"
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

# run_container prepends the Docker options to the original arguments. Stdin
# is attached only when it is not a terminal or another character device such
# as /dev/null. Attached, it would become a pipe inside the container, where
# tgsend would wait for a terminal's end of input and would no longer treat
# /dev/null as "no piped input"; unattached, the container sees /dev/null.
run_container() {
	set -- "$image" "$@"
	if [ -n "$mount_arg" ]; then
		set -- --mount "$mount_arg" "$@"
	fi
	if [ "$chat_id_set" -eq 1 ]; then
		set -- --env TGSEND_CHAT_ID "$@"
	fi
	if [ "$token_set" -eq 1 ]; then
		set -- --env TGSEND_TOKEN "$@"
	fi
	set -- --user "$(id -u):$(id -g)" --workdir "$workdir" --env "HOME=$home" "$@"
	if [ ! -t 0 ] && [ ! -c /dev/stdin ]; then
		set -- -i "$@"
	fi
	exec docker run --rm "$@"
}

run_container "$@"
