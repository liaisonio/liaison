#!/usr/bin/env bash

# The legacy global uninstaller cannot prove which instance owns a process or
# shared file. Fail closed before invoking any service or filesystem command.
printf '%s\n' \
  'The legacy global Edge uninstaller is disabled to protect other instances.' \
  'Use the console uninstall action for the specific connector when remote uninstall is enabled.' \
  'Otherwise ask an administrator to inspect that instance and remove only its verified service and files.' \
  'No services, processes, configuration or project files were changed.' >&2
exit 2
