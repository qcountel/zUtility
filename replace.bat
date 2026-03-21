@echo off
echo === FULL PROJECT REPLACE ===
git init
git remote add origin https://github.com/qcountel/zUtility.git
:: Принудительно выбираем ветку main
git branch -M main
:: Добавляем ВСЕ изменения (включая удаления)
git add -A
git commit -m "Full replace: %date% %time%"
:: Силовая отправка, затирающая старое содержимое GitHub
git push origin main --force
echo === DONE ===
pause