# Telegram sender

Voglio sviluppare un app golang (versione linguaggio latest) di nome `tgsend`
Quest'app mi consentirà di inviare messaggi al canale telegram configurato nel file `.tgsend`, in cui è presente anche il token.

## Esempi e2e

1. echo "Hello" | tgsend
   Qui il file di configurazione, non passato viene letto da ~/.tgsend

2. echo "Hello" | tgsend -c .tgsend
   Qui il file di configurazione viene passato esplicitamente

3. cat log.txt | tgsend

4. tgsend -c .tgsend -m "Hello"

5. tgsend -c .tgsend -m "Hello" --monospace
   Qui utilizza un carattere monospace

6. echo "Hello" | tgsend -c .tgsend --monospace --type INFO|WARNING|ERROR|CRITICAL
   qui viene specificato il tipo di messaggio (puoi usare delle icone rappresentative)

7. echo "Hello" | tgsend --title "My message title"
   qui viene specificato il titolo del messaggio

## Specifiche

- l'app deve essere corredata di test esaustivi
- deve essere preservata la formattazione originale dei messaggi.
- l'app deve essere architetturalmente modulare e facilmente estensibile

## Build e deploy

- deve essere prodotto un workflow github che fa test, builda, crea immagini docker e release tramite tag.
- deve essere prodotto il binario (per tutte le architetture (linux, mac, windows)
- deve essere prodotta un immagine docker ottimizzata a minimo peso, wrappabile in uno script sh
- deve essere prodotto uno script sh `tgsend.sh` che wrappa l'immagine docker e che abbia lo stesso comporramento e2e del binario

## Documentazione

- deve essere prodotto un readme per l'utente
- nel readme deve esserci anche una sezione specifica per l'utilizzo agentico. Do questa parte ad un LLM e lui mi notifica, ad esempio, durante un task lungo, lo stato di avanzamento.
- nel readme deve esserci anche un comando di installazione del binario con curl (del tipo curl https://... | sudo bash) che installa in /usr/local/bin
- lo stesso vale per lo script wrapper, sempre installabile come curl
- il repo remoto è https://github.com/manprint/tgsend.git. 
- puoi usare gh per pushare. 