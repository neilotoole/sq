#!/usr/bin/env sh

# Install oh-my-zsh
sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"

# Configure custom sq theme for oh-my-zsh
mkdir -p ~/.oh-my-zsh/custom/themes/
mv sq.zsh-theme ~/.oh-my-zsh/custom/themes/
sed -i /ZSH_THEME=/s/robbyrussell/sq/g ~/.zshrc


# Enable sq completion for oh-my-zsh
mkdir -p ~/.oh-my-zsh/plugins/sq/
sq completion zsh > ~/.oh-my-zsh/plugins/sq/_sq
sed -i /plugins=\(git\)/s/git/'git sq'/g ~/.zshrc

# Disable oh-my-zsh updates, because we don't want the
# update prompt to ever appear, blocking user work.
sed -i '/mode disabled/s/^# //g' ~/.zshrc

# Add some useful aliases.
cat << EOF >> ~/.zshrc

alias lsa="ls -AF -h --color=always -v --author --time-style=long-iso"
alias ll="ls -lAF -h --color=always -v --author --time-style=long-iso"
EOF


