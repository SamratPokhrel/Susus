import sys
import time
import requests
from colorama import Fore, Style, init

# Initialize Colorama
init(autoreset=True)

def check_website(url):
    try:
        response = requests.get(url)
        if response.status_code == 200:
            print(f"{Fore.GREEN}The website {url} is up!{Style.RESET_ALL}")
        else:
            print(f"{Fore.YELLOW}The website {url} is down! Status code: {response.status_code}{Style.RESET_ALL}")
    except requests.exceptions.RequestException as e:
        print(f"{Fore.RED}The website {url} is down! Error: {e}{Style.RESET_ALL}")

def main():
    if len(sys.argv) != 2:
        print(f"{Fore.RED}Usage: python name.py <url>{Style.RESET_ALL}")
        sys.exit(1)

    url = sys.argv[1]

    while True:
        check_website(url)
        time.sleep(0.8)

if __name__ == "__main__":
    main()
