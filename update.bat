@echo off
echo === UPDATE WITHOUT DELETING OLD ===
:: Синхронизация с сервером, чтобы не было конфликтов
git pull origin main --rebase
:: Добавление новых и измененных файлов
git add .
git commit -m "Update files: %date% %time%"
git push origin main
echo === DONE ===
pause